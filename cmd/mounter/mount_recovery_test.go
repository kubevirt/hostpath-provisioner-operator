/*
Copyright 2026 The hostpath provisioner operator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	mount "github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

// Model actual mount tables so successful commands become visible on the next read.
type mountRecoveryFake struct {
	host, container                     []*mount.Info
	expectedHost, expectedContainer     map[string]*mount.Info
	source                              *mount.Info
	uuid                                string
	events                              []string
	failMount, failOptions              string
	missingTarget, unavailableSource    string
	wrongDevice, ignoreMount, omitQuota bool
	locks, unlocks                      int
}

// Model a pool filesystem, its node binds, and the mounter's source binds.
// The workload mount is deliberately separate so cleanup cannot accidentally resurrect it.
func newMountRecoveryFake(fsType string) *mountRecoveryFake {
	f := &mountRecoveryFake{
		uuid:              "pool-filesystem-uuid",
		expectedHost:      make(map[string]*mount.Info),
		expectedContainer: make(map[string]*mount.Info),
	}
	for _, entry := range []struct{ path, root string }{
		{"/stage", "/"}, {"/var/pool", "/"}, {"/var/pool/child", "/child"},
		{"/var/lib/kubelet/pods/mounter/volumes/pool/mount", "/"},
		{"/var/lib/kubelet/pods/workload/volumes/pvc/mount", "/csi/pvc-workload"},
	} {
		info := quotaTestMount(entry.path, entry.root)
		info.FSType, info.VFSOptions = fsType, "rw,prjquota"
		f.host = append(f.host, info)
		copy := *info
		f.expectedHost[info.Mountpoint] = &copy
	}
	for i, entry := range []struct{ path, root string }{{"/source", "/"}, {"/source/child", "/child"}} {
		info := quotaTestMount(entry.path, entry.root)
		info.FSType, info.VFSOptions = fsType, "rw,prjquota"
		info.ID, info.Parent = i+2, i+1
		f.container = append(f.container, info)
		copy := *info
		f.expectedContainer[info.Mountpoint] = &copy
	}
	f.source = f.container[0]
	return f
}

// Replace every OS operation with in-memory mount tables and recorded commands.
func (f *mountRecoveryFake) ops() filesystemQuotaOps {
	return filesystemQuotaOps{
		readMounts: func(path string) ([]*mount.Info, error) {
			switch path {
			case "/host/proc/1/mountinfo":
				return append([]*mount.Info(nil), f.host...), nil
			case "/proc/self/mountinfo":
				return append([]*mount.Info(nil), f.container...), nil
			default:
				return nil, fmt.Errorf("unexpected mount table %s", path)
			}
		},
		readDeviceUUID: func(string) (string, error) { return f.uuid, nil },
		stat: func(path string, result *unix.Stat_t) error {
			if path == f.missingTarget {
				return os.ErrNotExist
			}
			if path == "/host/dev/rbd0" {
				result.Mode, result.Rdev = unix.S_IFBLK, unix.Mkdev(251, 0)
				if f.wrongDevice {
					result.Rdev = unix.Mkdev(251, 1)
				}
				return nil
			}
			result.Mode, result.Dev = unix.S_IFDIR, unix.Mkdev(8, 1)
			if path == f.unavailableSource {
				return nil
			}
			mounts := f.container
			if strings.HasPrefix(path, "/host/") {
				path, mounts = strings.TrimPrefix(path, "/host"), f.host
			}
			for _, info := range mounts {
				if pathWithin(path, info.Mountpoint) {
					result.Dev = unix.Mkdev(uint32(info.Major), uint32(info.Minor))
				}
			}
			return nil
		},
		runHost:  func(command string, args ...string) error { return f.run(false, command, args...) },
		runLocal: func(command string, args ...string) error { return f.run(true, command, args...) },
		lock: func() (func(), error) {
			f.locks++
			return func() { f.unlocks++ }, nil
		},
	}
}

// Successful fake mounts update the matching namespace; injected errors leave
// that state incomplete so the next recovery call has something real to retry.
func (f *mountRecoveryFake) run(local bool, command string, args ...string) error {
	namespace, mounts, expected := "host", &f.host, f.expectedHost
	if local {
		namespace, mounts, expected = "container", &f.container, f.expectedContainer
	}
	f.events = append(f.events, namespace+": "+command+" "+strings.Join(args, " "))
	if command != "/usr/bin/mount" {
		return fmt.Errorf("recovery must not unmount or format anything: %s", command)
	}
	target := args[len(args)-1]
	if args[0] == "-t" || args[0] == "--bind" {
		if target == f.failMount {
			return errors.New("mount failed")
		}
		if !f.ignoreMount {
			copy := *expected[target]
			if args[0] == "-t" {
				copy.VFSOptions = args[3]
				if f.omitQuota {
					copy.VFSOptions = "rw"
				}
			}
			*mounts = append(*mounts, &copy)
		}
	} else if args[0] == "-o" && target == f.failOptions {
		return errors.New("options failed")
	}
	return nil
}

func (f *mountRecoveryFake) capture(t *testing.T, state *poolMountRecovery) {
	t.Helper()
	if err := state.capture("/source", "/var/pool", "/host", f.source, true, f.ops()); err != nil {
		t.Fatal(err)
	}
	t.Logf("saved fake %s pool: %d host mounts and %d container mounts", f.source.FSType, len(state.saved.hostMounts), len(state.saved.containerMounts))
}

// Remove an entry from the observed table while leaving the saved recovery list intact.
func removeRecoveryMount(mounts []*mount.Info, path string) []*mount.Info {
	var result []*mount.Info
	for _, info := range mounts {
		if info.Mountpoint != path {
			result = append(result, info)
		}
	}
	return result
}

// Restore the host filesystem and nested binds before rebuilding container mounts.
// Repeat for ext4/XFS, and confirm a second healthy pass does not mount anything.
func TestPoolMountRecoveryOrder(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs"} {
		t.Run(fsType, func(t *testing.T) {
			f, state := newMountRecoveryFake(fsType), &poolMountRecovery{}
			f.capture(t, state)
			// Simulate every recorded mount disappearing after the snapshot was saved.
			f.host, f.container = nil, nil
			t.Log("recovering missing host mounts before the container source")
			if err := state.recover(); err != nil {
				t.Fatal(err)
			}
			var mounted []string
			for _, event := range f.events {
				if strings.Contains(event, "mount --bind") || strings.Contains(event, "mount -t") {
					fields := strings.Fields(event)
					mounted = append(mounted, fields[0]+fields[len(fields)-1])
				}
			}
			want := []string{"host:/stage", "host:/var/pool", "host:/var/pool/child", "host:/var/lib/kubelet/pods/mounter/volumes/pool/mount", "container:/source", "container:/source/child"}
			if !reflect.DeepEqual(mounted, want) {
				t.Fatalf("restore order = %v, want %v", mounted, want)
			}
			t.Logf("verified mount creation order: %v", mounted)
			if !strings.Contains(strings.Join(f.events, "\n"), "prjquota /dev/rbd0 /stage") {
				t.Fatal("filesystem must be recovered with project quotas")
			}
			f.events = nil
			if err := state.recover(); err != nil || len(f.events) != 0 {
				t.Fatalf("healthy mounts must be left alone: events=%v, error=%v", f.events, err)
			}
			if f.locks != f.unlocks {
				t.Fatal("recovery leaked the device lock")
			}
		})
	}
}

// Keep the complete desired list after a failed parent restore, then retry it.
func TestPoolMountRecoveryRetriesParentFailure(t *testing.T) {
	f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
	f.capture(t, state)
	f.host, f.container, f.failMount = nil, nil, "/stage"
	t.Log("forcing the filesystem mount to fail; no child bind should be attempted")
	if err := state.recover(); err == nil || len(f.events) != 1 {
		t.Fatalf("failed parent must block child mounts: events=%v, error=%v", f.events, err)
	}
	if len(state.saved.hostMounts) != 4 || len(state.saved.containerMounts) != 2 {
		t.Fatal("a failed recovery must not replace the desired list with the incomplete live table")
	}
	f.failMount = ""
	t.Log("clearing the injected failure and retrying the original saved layout")
	if err := state.recover(); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}

// When a bind succeeds but its options fail, retry the options without stacking another bind.
func TestPoolMountRecoveryRetriesOptions(t *testing.T) {
	f, state := newMountRecoveryFake("xfs"), &poolMountRecovery{}
	f.capture(t, state)
	f.container = removeRecoveryMount(f.container, "/source/child")
	f.failOptions = "/source/child"
	t.Log("creating the child bind successfully, then failing its mount options")
	if err := state.recover(); err == nil {
		t.Fatal("expected mount options failure")
	}
	f.failOptions = ""
	t.Log("retrying options only; creating a second bind would be a failure")
	if err := state.recover(); err != nil {
		t.Fatal(err)
	}
	binds := strings.Count(strings.Join(f.events, "\n"), "mount --bind /host/stage/child /source/child")
	if binds != 1 {
		t.Fatalf("expected one bind across both attempts, got %d: %v", binds, f.events)
	}
}

// Refuse changed devices, replaced mounts, vanished directories, and hidden child mounts.
func TestPoolMountRecoveryRefusesUnsafeChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*mountRecoveryFake)
		want   string
	}{
		{"different device", func(f *mountRecoveryFake) { f.container = nil; f.wrongDevice = true }, "device /dev/rbd0 changed"},
		{"reused device number", func(f *mountRecoveryFake) { f.container = nil; f.uuid = "another-volume" }, "UUID changed"},
		{"replacement mount", func(f *mountRecoveryFake) { f.container[0].Minor = 99 }, "different mount occupies"},
		{"stacked mounts", func(f *mountRecoveryFake) { f.container = append(f.container, f.container[0]) }, "stacked mounts"},
		{"parent missing with child present", func(f *mountRecoveryFake) { f.container = removeRecoveryMount(f.container, "/source") }, "cover nested mount"},
		{"directory deleted", func(f *mountRecoveryFake) { f.container = nil; f.missingTarget = "/source" }, "target /source is unavailable"},
		{"source filesystem unavailable", func(f *mountRecoveryFake) { f.container = nil; f.unavailableSource = "/host/stage" }, "unavailable pool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
			f.capture(t, state)
			t.Logf("injecting %s; expecting refusal containing %q and no mount commands", tc.name, tc.want)
			tc.change(f)
			err := state.recover()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if len(f.events) != 0 {
				t.Fatalf("unsafe recovery executed mount commands: %v", f.events)
			}
		})
	}
}

// Include the final pool bind after it is created, and keep copies of the original records.
func TestPoolMountRecoveryRecordsTargetAndKeepsSnapshot(t *testing.T) {
	f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
	f.host = removeRecoveryMount(f.host, "/var/pool")
	f.host = removeRecoveryMount(f.host, "/var/pool/child")
	f.capture(t, state)
	copy := *f.expectedHost["/var/pool"]
	f.host = append(f.host, &copy)
	if err := state.rememberTarget(); err != nil {
		t.Fatal(err)
	}
	copy.Root = "/changed-after-recording"
	f.host = removeRecoveryMount(f.host, "/var/pool")
	if err := state.recover(); err != nil {
		t.Fatalf("saved target was not recovered: %v", err)
	}
	if len(state.saved.hostMounts) != 3 {
		t.Fatalf("expected backing mounts and final target, got %d", len(state.saved.hostMounts))
	}
	for _, info := range state.saved.hostMounts {
		if strings.Contains(info.Mountpoint, "/workload/") {
			t.Fatal("must not record workload PVC mounts that can legitimately be removed")
		}
	}
}

// Multiple recovery callers must not race or create duplicate mounts.
func TestPoolMountRecoverySerializesCallers(t *testing.T) {
	f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
	f.capture(t, state)
	f.container = removeRecoveryMount(f.container, "/source/child")
	t.Log("starting concurrent recovery callers; only one should recreate the missing bind")
	var callers sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < cap(failures); i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			failures <- state.recover()
		}()
	}
	callers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Count(strings.Join(f.events, "\n"), "mount --bind") != 1 {
		t.Fatalf("concurrent recovery duplicated mounts: %v", f.events)
	}
}

// A failed initial capture must not become authoritative; a later healthy attempt can retry.
// Once captured, a changed pool identity must never overwrite the saved layout.
func TestPoolMountRecoveryCaptureValidation(t *testing.T) {
	f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
	f.wrongDevice = true
	if err := state.capture("/source", "/var/pool", "/host", f.source, true, f.ops()); err == nil || state.saved != nil {
		t.Fatalf("invalid device must not commit a recovery record: error=%v", err)
	}
	f.wrongDevice = false
	f.capture(t, state)
	changed := *f.source
	changed.Minor++
	if err := state.capture("/source", "/var/pool", "/host", &changed, true, f.ops()); err == nil {
		t.Fatal("changed pool identity replaced the saved layout")
	}
	f.host, f.container = nil, nil
	// Re-observing the same pool must not forget missing entries in the live table.
	f.capture(t, state)
	if err := state.recover(); err != nil {
		t.Fatal(err)
	}
}

// Block-backed pools recover their host filesystem without inventing a container /source bind.
func TestBlockPoolMountRecovery(t *testing.T) {
	f, state := newMountRecoveryFake("xfs"), &poolMountRecovery{}
	f.container = nil
	if err := state.capture("/source-device", "/var/pool", "/host", f.expectedHost["/var/pool"], false, f.ops()); err != nil {
		t.Fatal(err)
	}
	f.host = nil
	if err := state.recover(); err != nil {
		t.Fatal(err)
	}
	if len(state.saved.containerMounts) != 0 || strings.Contains(strings.Join(f.events, "\n"), "container:") {
		t.Fatalf("raw block source must not be recreated as a container filesystem: %v", f.events)
	}
}

// A snapshot captured before quota preparation must use prjquota after preparation succeeds.
func TestPoolMountRecoveryRemembersEnabledQuotas(t *testing.T) {
	f, state := newMountRecoveryFake("ext4"), &poolMountRecovery{}
	for _, info := range append(f.host, f.container...) {
		info.VFSOptions = "rw,noquota"
	}
	f.capture(t, state)
	state.enableQuota()
	f.host, f.container = nil, nil
	if err := state.recover(); err != nil {
		t.Fatal(err)
	}
	root := f.host[0]
	if !quotaEnabled(root) || strings.Contains(root.VFSOptions, "noquota") {
		t.Fatalf("recovery lost enabled quotas: %s", root.VFSOptions)
	}
}

// A successful command exit is insufficient: the expected mount and quota flags must appear.
func TestPoolMountRecoveryVerifiesResult(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*mountRecoveryFake)
		want      string
	}{
		{"mount still missing", func(f *mountRecoveryFake) { f.ignoreMount = true }, "still missing after recovery"},
		{"quota flag missing", func(f *mountRecoveryFake) { f.omitQuota = true }, "quotas are missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, state := newMountRecoveryFake("xfs"), &poolMountRecovery{}
			f.capture(t, state)
			f.host = nil
			tc.configure(f)
			err := state.recover()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if strings.Contains(strings.Join(f.events, "\n"), "--bind") {
				t.Fatalf("must not recover child mounts after filesystem verification fails: %v", f.events)
			}
		})
	}
}
