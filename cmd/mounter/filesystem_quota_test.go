/*
Copyright 2021 The hostpath provisioner operator Authors.

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
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mount "github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

func quotaTestMount(path, root string) *mount.Info {
	return &mount.Info{Mountpoint: path, Root: root, Major: 251, Minor: 0, FSType: "ext4", Source: "/dev/rbd0", Options: "rw,relatime", VFSOptions: "rw,stripe=1024"}
}

// Check that parents are restored before children and unsafe mount layouts are rejected.
func TestQuotaMountPlan(t *testing.T) {
	root := quotaTestMount("/staging", "/")
	parent := quotaTestMount("/pool", "/volumes")
	child := quotaTestMount("/pool/nested", "/volumes/nested")
	plan, err := quotaMountPlan([]*mount.Info{child, parent, root}, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, []*mount.Info{root, parent, child}) {
		t.Fatalf("expected filesystem root, parent bind, child bind; got %v", plan)
	}

	for _, tc := range []struct {
		name   string
		mounts []*mount.Info
	}{
		// Never prepare quotas by unmounting the node root filesystem.
		{"node root", []*mount.Info{quotaTestMount("/", "/")}},
		// Two mounts at the same path cannot be restored safely by this plan.
		{"stacked mounts", []*mount.Info{root, root}},
		// A subdirectory bind alone is insufficient to rebuild the filesystem mount.
		{"no filesystem root", []*mount.Info{parent}},
		// Reject nested storage that belongs to a different filesystem.
		{"foreign nested mount", []*mount.Info{root, parent, {Mountpoint: "/pool/tmp", Major: 0, Minor: 55}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := quotaMountPlan(tc.mounts, root); err == nil {
				t.Fatal("expected an unsafe mount layout to be rejected")
			}
		})
	}
}

// Check child-first unmounting and parent-first restoration, including recovery after failures.
func TestQuotaMountTransaction(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failUnmount string
		failPrepare bool
		wantPrepare bool
		wantUnmount []string
		wantRestore []string
	}{
		// Unmount all children before the filesystem, then restore in the opposite order.
		{"success", "", false, true, []string{"/pool/child", "/pool", "/stage"}, []string{"/stage", "/pool", "/pool/child"}},
		// If a parent cannot be unmounted, restore the child already removed.
		{"busy parent", "/pool", false, false, []string{"/pool/child", "/pool"}, []string{"/pool/child"}},
		// If the filesystem stays busy, restore its removed binds without preparing quotas.
		{"busy filesystem", "/stage", false, false, []string{"/pool/child", "/pool", "/stage"}, []string{"/pool", "/pool/child"}},
		// Restore all saved mounts with their original options when preparation fails.
		{"preparation failed", "", true, true, []string{"/pool/child", "/pool", "/stage"}, []string{"/stage", "/pool", "/pool/child"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := []*mount.Info{quotaTestMount("/stage", "/"), quotaTestMount("/pool", "/"), quotaTestMount("/pool/child", "/child")}
			var unmounted, restored []string
			prepared := false
			err := withQuotaMountsDetached(plan, "/dev/rbd0", func(command string, args ...string) error {
				if command == "/usr/bin/umount" {
					unmounted = append(unmounted, args[0])
					if args[0] == tc.failUnmount {
						return errors.New("busy")
					}
				} else if args[0] == "-t" || args[0] == "--bind" {
					restored = append(restored, args[len(args)-1])
					if args[0] == "-t" {
						hasQuota := strings.Contains(strings.Join(args, ","), "prjquota")
						if hasQuota != (!tc.failPrepare && tc.wantPrepare) {
							t.Errorf("wrong quota mount options: %v", args)
						}
					}
				}
				return nil
			}, func() error {
				prepared = true
				if tc.failPrepare {
					return errors.New("still mounted in another namespace")
				}
				return nil
			})
			if (err != nil) != (tc.failUnmount != "" || tc.failPrepare) {
				t.Errorf("unexpected error: %v", err)
			}
			if prepared != tc.wantPrepare {
				t.Errorf("prepare called = %t", prepared)
			}
			if !reflect.DeepEqual(unmounted, tc.wantUnmount) {
				t.Errorf("unmount order %v, want %v", unmounted, tc.wantUnmount)
			}
			if !reflect.DeepEqual(restored, tc.wantRestore) {
				t.Errorf("restore order %v, want %v", restored, tc.wantRestore)
			}
		})
	}
}

// Do not recreate bind mounts when their backing filesystem could not be restored.
func TestQuotaMountRestoreFailure(t *testing.T) {
	root := quotaTestMount("/stage", "/")
	var binds int
	err := withQuotaMountsDetached([]*mount.Info{root, quotaTestMount("/pool", "/")}, "/dev/rbd0", func(command string, args ...string) error {
		if command == "/usr/bin/mount" && args[0] == "-t" {
			return errors.New("mount failed")
		}
		if command == "/usr/bin/mount" && args[0] == "--bind" {
			binds++
		}
		return nil
	}, func() error { return nil })
	if err == nil || binds != 0 {
		t.Fatalf("must not bind the underlying empty directory after root mount failure: err=%v, binds=%d", err, binds)
	}
}

// Host mounts use the same propagation restoration as container binds.
// A mount must regain its upstream relationship before sharing is enabled downstream.
func TestQuotaHostMountRestoresUpstreamPropagationBeforeSharing(t *testing.T) {
	root := quotaTestMount("/stage", "/")
	root.Optional = "shared:726 master:100"
	var propagationFlags []string
	err := withQuotaMountsDetached([]*mount.Info{root}, "/dev/rbd0", func(command string, args ...string) error {
		if command == "/usr/bin/mount" && strings.HasPrefix(args[0], "--make-") {
			propagationFlags = append(propagationFlags, args[0])
		}
		return nil
	}, func() error { return nil })
	want := []string{"--make-slave", "--make-shared"}
	if err != nil || !reflect.DeepEqual(propagationFlags, want) {
		t.Fatalf("host propagation flags = %v, want %v; error = %v", propagationFlags, want, err)
	}
}

// Recognize project quota enforcement flags; accounting-only or disabled quotas are not sufficient.
func TestQuotaEnabled(t *testing.T) {
	for _, tc := range []struct {
		options string
		want    bool
	}{
		{"rw,prjquota", true}, {"rw,pquota", true}, {"rw,pqnoenforce", false}, {"rw,noquota", false}, {"rw", false},
	} {
		if got := quotaEnabled(&mount.Info{VFSOptions: tc.options}); got != tc.want {
			t.Errorf("quotaEnabled(%q) = %t", tc.options, got)
		}
	}
}

// Detect mounts held in another process namespace, even when the device has a different alias.
func TestQuotaDeviceUnmountedChecksOtherNamespaces(t *testing.T) {
	host := t.TempDir()
	proc := filepath.Join(host, "proc", "123")
	if err := os.MkdirAll(proc, 0755); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(proc, "mountinfo")
	if err := os.WriteFile(table, []byte("10 1 251:0 / /hidden rw - ext4 /dev/other-alias rw\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := quotaDeviceUnmounted(host, quotaTestMount("/source", "/")); err == nil || !strings.Contains(err.Error(), "/hidden") {
		t.Fatalf("expected hidden device reference, got %v", err)
	}
	if err := os.WriteFile(table, []byte("10 1 252:1 / / rw - xfs /dev/vda1 rw\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := quotaDeviceUnmounted(host, quotaTestMount("/source", "/")); err != nil {
		t.Fatal(err)
	}
}

// Keep existing mount settings while enabling quotas, and preserve the original flags for rollback.
func TestQuotaMountOptionsPreserveExistingSettings(t *testing.T) {
	info := quotaTestMount("/stage", "/")
	info.VFSOptions += ",noquota"
	options := quotaMountOptions(info, true)
	for _, want := range []string{"rw", "relatime", "stripe=1024", "prjquota"} {
		if !strings.Contains(","+options+",", ","+want+",") {
			t.Errorf("missing %s in %s", want, options)
		}
	}
	if strings.Contains(options, "noquota") {
		t.Errorf("conflicting quota options: %s", options)
	}
	if !strings.Contains(quotaMountOptions(info, false), "noquota") {
		t.Fatal("rollback must preserve the original quota settings")
	}
}

// This fake records both namespaces without executing mount or tune2fs.
type filesystemQuotaFake struct {
	source               *mount.Info
	hostMount            *mount.Info
	localMounts          []*mount.Info
	events               []string
	failTool             bool
	failHostMountView    bool
	failUnmount          bool
	failPrepare          bool
	failContainerUnmount bool
	failVerify           bool
	wrongDevice          bool
	hiddenMount          bool
	lockBusy             bool
	locked               bool
	unlocked             bool
	mounted              bool
	quotaMounted         bool
	reads                int
	rootPropagation      string
	sameNamespace        bool
	failIsolation        bool
}

func newFilesystemQuotaFake(fsType string) *filesystemQuotaFake {
	source := quotaTestMount("/source", "/")
	source.FSType = fsType
	source.ID, source.Parent = 2, 1
	root := quotaTestMount("/staging", "/")
	root.FSType = fsType
	return &filesystemQuotaFake{source: source, hostMount: root, localMounts: []*mount.Info{source}, mounted: true}
}

func (f *filesystemQuotaFake) ops() filesystemQuotaOps {
	return filesystemQuotaOps{
		readMounts: func(path string) ([]*mount.Info, error) {
			if path == "/proc/self/mountinfo" {
				containerRoot := &mount.Info{ID: 1, Parent: 1, Mountpoint: "/", Root: "/", Major: 0, Minor: 99, Optional: f.rootPropagation}
				return append([]*mount.Info{containerRoot}, f.localMounts...), nil
			}
			if path != "/host/proc/1/mountinfo" {
				return nil, fmt.Errorf("unexpected mount table %s", path)
			}
			f.reads++
			root := *f.hostMount
			if f.quotaMounted && !f.failVerify {
				root.VFSOptions += ",prjquota"
			}
			return []*mount.Info{&root}, nil
		},
		stat: func(path string, st *unix.Stat_t) error {
			if path == "/proc/self/ns/mnt" || path == "/host/proc/1/ns/mnt" {
				st.Dev, st.Ino = 1, 100
				if path == "/host/proc/1/ns/mnt" && !f.sameNamespace {
					st.Ino = 200
				}
				return nil
			}
			if path == "/host/dev/rbd0" {
				st.Mode = unix.S_IFBLK
				st.Rdev = unix.Mkdev(251, 0)
				if f.wrongDevice {
					st.Rdev = unix.Mkdev(251, 1)
				}
				return nil
			}
			st.Dev = unix.Mkdev(251, 0)
			if !f.mounted {
				st.Dev = unix.Mkdev(8, 1)
			}
			return nil
		},
		runHost: func(command string, args ...string) error {
			f.events = append(f.events, "host: "+command+" "+strings.Join(args, " "))
			switch command {
			case "/usr/bin/test":
				if f.failTool {
					return errors.New("tool missing")
				}
			case "/usr/bin/findmnt":
				if f.failHostMountView {
					return errors.New("mount not visible")
				}
			case "/usr/bin/umount":
				if f.failUnmount {
					return errors.New("device busy")
				}
				f.mounted = false
			case "/usr/sbin/tune2fs":
				if f.mounted {
					return errors.New("tune2fs called on mounted device")
				}
				if f.failPrepare {
					return errors.New("tune2fs failed")
				}
			case "/usr/bin/mount":
				if len(args) > 0 && args[0] == "-t" {
					f.mounted = true
					f.quotaMounted = strings.Contains(strings.Join(args, ","), "prjquota")
				}
			}
			return nil
		},
		runLocal: func(command string, args ...string) error {
			f.events = append(f.events, "local: "+command+" "+strings.Join(args, " "))
			if len(args) > 0 && args[0] == "--make-slave" && f.failIsolation {
				return errors.New("isolation failed")
			}
			if command == "/usr/bin/umount" && f.failContainerUnmount {
				return errors.New("container mount busy")
			}
			return nil
		},
		lock: func() (func(), error) {
			if f.lockBusy {
				return nil, errors.New("lock busy")
			}
			f.locked = true
			return func() { f.unlocked = true }, nil
		},
		checkUnmounted: func() error {
			f.events = append(f.events, "check namespaces")
			if f.hiddenMount {
				return errors.New("device still mounted in another namespace")
			}
			return nil
		},
	}
}

// Exercise the full ext4 and XFS setup flow with fake host operations.
// Verify preparation, quota mount flags, container restoration, and repeated calls.
func TestEnableFilesystemPoolQuota(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs"} {
		t.Run(fsType, func(t *testing.T) {
			f := newFilesystemQuotaFake(fsType)
			if err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops()); err != nil {
				t.Fatal(err)
			}
			if !f.locked || !f.unlocked || !f.mounted || !f.quotaMounted {
				t.Fatalf("setup did not leave quotas mounted and release lock: %+v", f)
			}
			joined := strings.Join(f.events, "\n")
			wantMount := "host: /usr/bin/mount -t " + fsType + " -o rw,relatime,stripe=1024,prjquota /dev/rbd0 /staging"
			if !strings.Contains(joined, wantMount) {
				t.Fatalf("missing quota mount %q in:\n%s", wantMount, joined)
			}
			wantBind := "local: /usr/bin/mount --bind /host/staging /source"
			if !strings.Contains(joined, wantBind) {
				t.Fatalf("container source was not restored:\n%s", joined)
			}
			tune := "host: /usr/sbin/tune2fs -O project,quota -Q prjquota /dev/rbd0"
			if strings.Contains(joined, tune) != (fsType == "ext4") {
				t.Fatalf("wrong filesystem preparation:\n%s", joined)
			}
			if strings.Contains(joined, "mkfs") {
				t.Fatal("existing filesystem must never be formatted")
			}
			if fsType == "ext4" && !(strings.Index(joined, "check namespaces") < strings.Index(joined, tune) && strings.Index(joined, tune) < strings.Index(joined, wantMount)) {
				t.Fatalf("ext4 preparation must occur offline before mount:\n%s", joined)
			}
			// Simulate the next mounter iteration: enabled quotas must not trigger another unmount.
			f.source.VFSOptions += ",prjquota"
			f.events = nil
			if err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops()); err != nil {
				t.Fatal(err)
			}
			if len(f.events) != 0 {
				t.Fatalf("already enabled quota changed mounts: %v", f.events)
			}
		})
	}
}

// Inject setup failures and check that errors are reported, removed mounts are restored, and locks are released.
func TestEnableFilesystemPoolQuotaFailures(t *testing.T) {
	for _, tc := range []struct {
		name            string
		configure       func(*filesystemQuotaFake)
		wantError       string
		wantHostUnmount bool
		wantLocalBind   bool
	}{
		// Check required host tools before removing any mounts.
		{"missing tools", func(f *filesystemQuotaFake) { f.failTool = true }, "tool missing", false, false},
		// Detect a wrong namespace/root view before detaching any container or host mounts.
		{"wrong host mount view", func(f *filesystemQuotaFake) { f.failHostMountView = true }, "host command cannot see planned mount", false, false},
		// Reject a device whose major/minor numbers do not match the pool.
		{"wrong device", func(f *filesystemQuotaFake) { f.wrongDevice = true }, "does not match pool block device", false, false},
		// Do not touch mounts while another mounter holds the device lock.
		{"lock busy", func(f *filesystemQuotaFake) { f.lockBusy = true }, "lock busy", false, false},
		// Stop if the container source cannot be unmounted.
		{"source busy", func(f *filesystemQuotaFake) { f.failContainerUnmount = true }, "container mount busy", false, false},
		// Restore the container source if the host mount cannot be released.
		{"host mount busy", func(f *filesystemQuotaFake) { f.failUnmount = true }, "device busy", true, true},
		// Do not run tune2fs while another namespace still holds the filesystem mounted.
		{"hidden mount", func(f *filesystemQuotaFake) { f.hiddenMount = true }, "another namespace", true, true},
		// Restore the filesystem with its saved options if ext4 preparation fails.
		{"tune2fs failed", func(f *filesystemQuotaFake) { f.failPrepare = true }, "tune2fs failed", true, true},
		// Report failure when the restored filesystem does not expose quota flags.
		{"verification failed", func(f *filesystemQuotaFake) { f.failVerify = true }, "quotas are missing", true, true},
		// Failed isolation must stop setup before any unmounts.
		{"parent isolation failed", func(f *filesystemQuotaFake) { f.rootPropagation = "shared:100"; f.failIsolation = true }, "isolation failed", false, false},
		// Namespace isolation must never change propagation on the node itself.
		{"host namespace", func(f *filesystemQuotaFake) { f.rootPropagation = "shared:100"; f.sameNamespace = true }, "host mount namespace", false, false},
		// Stop before unmounting host paths if the container source is missing.
		{"source disappeared", func(f *filesystemQuotaFake) { f.localMounts = nil }, "cannot find mounter source", false, false},
		// Reject multiple container mounts layered at the same source path.
		{"stacked source", func(f *filesystemQuotaFake) { f.localMounts = append(f.localMounts, f.source) }, "stacked container mounts", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFilesystemQuotaFake("ext4")
			tc.configure(f)
			err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops())
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantError)
			}
			joined := strings.Join(f.events, "\n")
			if strings.Contains(joined, "host: /usr/bin/umount") != tc.wantHostUnmount {
				t.Fatalf("unexpected host unmounts:\n%s", joined)
			}
			if strings.Contains(joined, "local: /usr/bin/mount --bind") != tc.wantLocalBind {
				t.Fatalf("unexpected container restoration:\n%s", joined)
			}
			if !f.mounted {
				t.Fatalf("failed setup left host filesystem unmounted:\n%s", joined)
			}
			if f.locked && !f.unlocked {
				t.Fatal("setup leaked its device lock")
			}
			if tc.name == "hidden mount" && strings.Contains(joined, "host: /usr/sbin/tune2fs") {
				t.Fatal("must not run tune2fs while any namespace holds the device")
			}
			if tc.name == "tune2fs failed" && f.quotaMounted {
				t.Fatal("preparation failure must restore original mount options")
			}
		})
	}
}

// If a child unmounts but its parent stays busy, put back only the child.
// The host filesystem must remain untouched during this partial container failure.
func TestFilesystemQuotaPartialContainerUnmount(t *testing.T) {
	f := newFilesystemQuotaFake("ext4")
	child := quotaTestMount("/source/child", "/child")
	child.ID, child.Parent = 3, f.source.ID
	f.localMounts = append(f.localMounts, child)
	system := f.ops()
	runLocal := system.runLocal
	busy := errors.New("source still busy")
	system.runLocal = func(command string, args ...string) error {
		if err := runLocal(command, args...); err != nil {
			return err
		}
		if command == "/usr/bin/umount" && args[0] == "/source" {
			return busy
		}
		return nil
	}
	err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, system)
	if !errors.Is(err, busy) {
		t.Fatalf("expected busy parent error, got %v", err)
	}
	var restored []string
	for _, event := range f.events {
		if strings.HasPrefix(event, "host: /usr/bin/umount") {
			t.Fatalf("host mount was touched after partial container failure: %s", event)
		}
		if strings.HasPrefix(event, "local: /usr/bin/mount --bind") {
			restored = append(restored, event)
		}
	}
	want := []string{"local: /usr/bin/mount --bind /host/staging/child /source/child"}
	if !reflect.DeepEqual(restored, want) || !f.unlocked {
		t.Fatalf("expected only child restoration and released lock; restored=%v, unlocked=%t", restored, f.unlocked)
	}
}

// Keep both errors when quota preparation fails and restoring a container bind also fails.
func TestFilesystemQuotaPreservesSetupAndRestorationErrors(t *testing.T) {
	f := newFilesystemQuotaFake("ext4")
	system := f.ops()
	prepareErr := errors.New("preparation failed")
	restoreErr := errors.New("container restoration failed")
	runHost, runLocal := system.runHost, system.runLocal
	system.runHost = func(command string, args ...string) error {
		if command == "/usr/sbin/tune2fs" {
			return prepareErr
		}
		return runHost(command, args...)
	}
	system.runLocal = func(command string, args ...string) error {
		if command == "/usr/bin/mount" && args[0] == "--bind" {
			return restoreErr
		}
		return runLocal(command, args...)
	}
	err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, system)
	if !errors.Is(err, prepareErr) || !errors.Is(err, restoreErr) {
		t.Fatalf("both preparation and restoration errors must survive: %v", err)
	}
	if !f.mounted || f.quotaMounted || !f.unlocked {
		t.Fatalf("failed setup must restore original host options and release lock: %+v", f)
	}
}

// Leave unsupported filesystems and the node root filesystem unchanged.
func TestEnableFilesystemPoolQuotaSkipsUnsupportedAndRoot(t *testing.T) {
	for _, fsType := range []string{"nfs", "ceph", "ext4", "xfs"} {
		t.Run(fsType, func(t *testing.T) {
			f := newFilesystemQuotaFake(fsType)
			f.hostMount.Mountpoint = "/"
			if err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops()); err != nil {
				t.Fatal(err)
			}
			if len(f.events) != 0 {
				t.Fatalf("must not change unsupported or node root filesystems: %v", f.events)
			}
		})
	}
}

// Prevent concurrent preparation of the same device while allowing other devices and later retries.
func TestFilesystemQuotaSetupLock(t *testing.T) {
	host := t.TempDir()
	source := quotaTestMount("/source", "/")
	release, err := lockFilesystemQuotaSetup(host, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if release != nil {
			release()
		}
	})
	if unlock, err := lockFilesystemQuotaSetup(host, source); err == nil {
		unlock()
		t.Fatal("a second setup must not acquire the same device lock")
	}
	other := *source
	other.Minor++
	unlockOther, err := lockFilesystemQuotaSetup(host, &other)
	if err != nil {
		t.Fatalf("independent filesystems should have independent locks: %v", err)
	}
	unlockOther()
	release()
	release = nil
	unlockAgain, err := lockFilesystemQuotaSetup(host, source)
	if err != nil {
		t.Fatalf("lock must be reusable after setup: %v", err)
	}
	unlockAgain()
}

// Replace conflicting or duplicate project quota flags with exactly one prjquota enforcement flag.
func TestQuotaMountOptionsNormalizeProjectFlags(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs"} {
		for _, options := range []string{"rw,noquota", "rw,pqnoenforce", "rw,prjquota", "rw,pquota", "rw,prjquota,pqnoenforce"} {
			t.Run(fsType+"/"+options, func(t *testing.T) {
				info := quotaTestMount("/stage", "/")
				info.FSType = fsType
				info.VFSOptions = options
				normalized := quotaMountOptions(info, true)
				if strings.Count(normalized, "prjquota") != 1 {
					t.Errorf("expected one project enforcement flag: %s", normalized)
				}
				for _, bad := range []string{"noquota", "pqnoenforce", "pquota"} {
					if strings.Contains(","+normalized+",", ","+bad+",") {
						t.Errorf("conflicting flag %s in %s", bad, normalized)
					}
				}
			})
		}
	}
}

// A shared source under the private container root must reach quota preparation.
// Check both successful setup and rollback without changing propagation before unmount.
func TestFilesystemQuotaSharedSource(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs"} {
		for _, failure := range []string{"", "source busy", "preparation failed"} {
			t.Run(fsType+"/"+failure, func(t *testing.T) {
				f := newFilesystemQuotaFake(fsType)
				f.source.Optional = "shared:726"
				f.failContainerUnmount = failure == "source busy"
				f.failPrepare = failure == "preparation failed"
				// Namespace checks fail for either filesystem when testing rollback.
				f.hiddenMount = failure == "preparation failed"
				err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops())
				if (err != nil) != (failure != "") {
					t.Fatalf("unexpected setup result: %v", err)
				}
				joined := strings.Join(f.events, "\n")
				if !strings.Contains(joined, "local: /usr/bin/umount /source") {
					t.Fatalf("shared source was rejected before unmount:\n%s", joined)
				}
				if strings.Contains(joined, "--make-private /source") || strings.Contains(joined, "--make-slave /source") {
					t.Fatalf("must not isolate a source with a private parent:\n%s", joined)
				}
				shared := "local: /usr/bin/mount --make-shared /source"
				if strings.Contains(joined, shared) != (failure != "source busy") {
					t.Fatalf("unexpected shared mode restoration:\n%s", joined)
				}
				if failure == "source busy" && strings.Contains(joined, "host: /usr/bin/umount") {
					t.Fatal("host mounts must remain untouched when the container source is busy")
				}
			})
		}
	}
}

// Isolate shared parents using mount IDs; a source's own shared flag is not enough.
// Missing parents must stop setup before any propagation changes are made.
func TestQuotaUnmountParents(t *testing.T) {
	for _, tc := range []struct {
		name, parentOptions, childOptions string
		missingParent, wantIsolation      bool
	}{
		{"shared target private parent", "", "shared:726", false, false},
		{"shared target receiving parent", "master:100", "shared:726", false, false},
		{"private target shared parent", "shared:100", "", false, true},
		{"shared target shared parent", "shared:100", "shared:726", false, true},
		{"missing parent", "", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := &mount.Info{ID: 1, Mountpoint: "/", Optional: tc.parentOptions}
			child := &mount.Info{ID: 2, Parent: 1, Mountpoint: "/source", Optional: tc.childOptions}
			all := []*mount.Info{parent, child}
			if tc.missingParent {
				all = all[1:]
			}
			f := newFilesystemQuotaFake("ext4")
			err := isolateQuotaUnmountParents(all, []*mount.Info{child}, "/host", f.ops())
			if (err != nil) != tc.missingParent {
				t.Fatalf("unexpected parent isolation result: %v", err)
			}
			isolated := strings.Contains(strings.Join(f.events, "\n"), "--make-slave /")
			if isolated != tc.wantIsolation {
				t.Fatalf("parent isolated = %t, want %t; commands: %v", isolated, tc.wantIsolation, f.events)
			}
		})
	}
}

// Restore upstream propagation before downstream sharing, and retain private mount behavior.
func TestRestoreQuotaPropagation(t *testing.T) {
	for _, tc := range []struct {
		options string
		flags   []string
	}{
		{"shared:726", []string{"--make-shared"}},
		{"master:100", []string{"--make-slave"}},
		{"shared:726 master:100", []string{"--make-slave", "--make-shared"}},
		{"", []string{"--make-private"}},
		{"unbindable", []string{"--make-unbindable"}},
	} {
		t.Run(tc.options, func(t *testing.T) {
			var got []string
			err := restoreQuotaPropagation(&mount.Info{Mountpoint: "/source", Optional: tc.options}, func(command string, args ...string) error { got = append(got, args[0]); return nil })
			if err != nil || !reflect.DeepEqual(got, tc.flags) {
				t.Fatalf("flags=%v, want %v; error=%v", got, tc.flags, err)
			}
		})
	}
}

// Match the reported cluster layout: both the container root and /source are shared.
// Isolate the root non-recursively before unmounting, leaving /host propagation alone.
func TestFilesystemQuotaSharedContainerRoot(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs"} {
		for _, busy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/busy=%t", fsType, busy), func(t *testing.T) {
				f := newFilesystemQuotaFake(fsType)
				f.rootPropagation = "shared:50"
				f.source.Optional = "shared:727"
				f.failContainerUnmount = busy
				err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops())
				if (err != nil) != busy {
					t.Fatalf("unexpected setup result: %v", err)
				}
				joined := strings.Join(f.events, "\n")
				isolate := strings.Index(joined, "local: /usr/bin/mount --make-slave /\n")
				unmount := strings.Index(joined, "local: /usr/bin/umount /source")
				if isolate < 0 || unmount <= isolate {
					t.Fatalf("must isolate root before unmounting source:\n%s", joined)
				}
				if strings.Contains(joined, "--make-rslave") || strings.Contains(joined, "--make-slave /host") {
					t.Fatalf("must not recursively change /host propagation:\n%s", joined)
				}
				if busy && strings.Contains(joined, "host: /usr/bin/umount") {
					t.Fatal("must preserve host mounts when source is busy")
				}
			})
		}
	}
}

// Nested shared mounts need their own parents isolated before child-first unmounting.
func TestFilesystemQuotaNestedSharedParents(t *testing.T) {
	f := newFilesystemQuotaFake("ext4")
	f.rootPropagation = "shared:50"
	f.source.Optional = "shared:727"
	child := quotaTestMount("/source/child", "/child")
	child.ID, child.Parent, child.Optional = 3, f.source.ID, "shared:728"
	f.localMounts = append(f.localMounts, child)
	if err := enableFilesystemPoolQuotaWithOps("/source", "/host", f.source, f.ops()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(f.events, "\n")
	isolation := strings.Index(joined, "local: /usr/bin/mount --make-slave /source")
	childUnmount := strings.Index(joined, "local: /usr/bin/umount /source/child")
	parentUnmount := strings.Index(joined, "local: /usr/bin/umount /source\n")
	if isolation < 0 || childUnmount <= isolation || parentUnmount <= childUnmount {
		t.Fatalf("incorrect nested mount isolation/unmount order:\n%s", joined)
	}
	if child.Optional != "shared:728" || f.source.Optional != "shared:727" {
		t.Fatal("isolation must not overwrite the saved propagation records")
	}
}

// Regress the live failure: /host and the container's / are not the host process's root mount.
// Preserve command arguments and errors while entering the host's mount, root, and working directory.
func TestHostQuotaCommandUsesHostProcessRoot(t *testing.T) {
	for _, hostPath := range []string{"/host", "/node-root"} {
		t.Run(hostPath, func(t *testing.T) {
			sentinel := errors.New("command failed")
			var gotCommand string
			var gotArgs []string
			run := hostQuotaCommand(hostPath, func(command string, args ...string) error {
				gotCommand = command
				gotArgs = append([]string(nil), args...)
				return sentinel
			})
			err := run("/usr/bin/umount", "/pool path/staging")
			root := hostPath + "/proc/1/root"
			want := []string{"--mount=" + hostPath + "/proc/1/ns/mnt", "--root=" + root, "--wd=" + root, "--", "/usr/bin/umount", "/pool path/staging"}
			if gotCommand != "/usr/bin/nsenter" || !reflect.DeepEqual(gotArgs, want) {
				t.Fatalf("unexpected namespace invocation: %s %q; want %q", gotCommand, gotArgs, want)
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("command failure must reach the caller: %v", err)
			}
		})
	}
}
