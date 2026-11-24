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
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	mount "github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

// Keep desired mounts independently of the live mount table. This record lasts
// for this process only; a missing mount must never erase its saved entry.
var rememberedPoolMounts poolMountRecovery

// One mounter owns one pool. The mutex protects its saved layout and retries;
// the filesystem lock separately coordinates changes with other mounter processes.
type poolMountRecovery struct {
	mu     sync.Mutex
	saved  *poolMountSnapshot
	system filesystemQuotaOps
	// A mount may exist while its options or propagation still need to be restored.
	pendingSetup map[string]bool
}

// Keep expected host and container paths, mount settings, and device identity.
// Both mount lists are ordered so their parents can be restored before children.
type poolMountSnapshot struct {
	sourcePath, targetPath, hostPath string
	source                           mount.Info
	hostMounts, containerMounts      []*mount.Info
	device, deviceUUID               string
	enforceQuota                     bool
}

// Read the filesystem UUID on the node, where the real block device is visible.
func readPoolDeviceUUID(hostPath, device string) (string, error) {
	var output []byte
	run := hostQuotaCommand(hostPath, func(command string, args ...string) error {
		var err error
		output, err = exec.Command(command, args...).Output()
		return err
	})
	if err := run("/usr/sbin/blkid", "-s", "UUID", "-o", "value", device); err != nil {
		return "", fmt.Errorf("failed to read filesystem UUID for %s: %w", device, err)
	}
	uuid := strings.TrimSpace(string(output))
	if uuid == "" {
		return "", fmt.Errorf("filesystem UUID is missing for %s", device)
	}
	return uuid, nil
}

// Capture once, before quota setup detaches anything. Save copies so later mount
// table reads or partial failures cannot change the desired layout.
func (r *poolMountRecovery) capture(sourcePath, targetPath, hostPath string, source *mount.Info, watchContainer bool, system filesystemQuotaOps) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved != nil {
		if r.saved.sourcePath != sourcePath || r.saved.targetPath != targetPath || r.saved.hostPath != hostPath || !samePoolMount(&r.saved.source, source) {
			return fmt.Errorf("pool identity changed; refusing to replace the saved mount layout")
		}
		return nil
	}
	if source.FSType != "ext4" && source.FSType != "xfs" {
		log.V(3).Info("Skipping mount recovery tracking for unsupported filesystem", "filesystem", source.FSType, "sourcePath", sourcePath)
		return nil
	}
	if !filepath.IsAbs(sourcePath) || filepath.Clean(sourcePath) == "/" || !filepath.IsAbs(targetPath) || filepath.Clean(targetPath) == "/" {
		return fmt.Errorf("invalid pool paths for mount recovery: source=%q target=%q", sourcePath, targetPath)
	}
	unlock, err := system.lock()
	if err != nil {
		return err
	}
	defer unlock()
	hostMounts, err := system.readMounts(filepath.Join(hostPath, "proc/1/mountinfo"))
	if err != nil {
		return err
	}
	for _, info := range hostMounts {
		if info.Mountpoint == "/" && info.Major == source.Major && info.Minor == source.Minor {
			log.V(3).Info("Leaving node root filesystem outside mount recovery", "sourcePath", sourcePath)
			return nil // Leave pools on the node root filesystem to the existing mount flow.
		}
	}
	hostPlan, err := quotaMountPlan(hostMounts, source)
	if err != nil {
		return err
	}
	var containerPlan []*mount.Info
	// Filesystem pools need their /source bind restored too; raw block pools do not.
	if watchContainer {
		containerMounts, err := system.readMounts("/proc/self/mountinfo")
		if err != nil {
			return err
		}
		containerPlan, err = containerQuotaMountPlan(containerMounts, sourcePath, source)
		if err != nil {
			return err
		}
	}
	device := strings.TrimPrefix(hostPlan[0].Source, rhcosPrefix)
	var deviceInfo unix.Stat_t
	if err := system.stat(filepath.Join(hostPath, device), &deviceInfo); err != nil {
		return err
	}
	if !matchesPoolDevice(&deviceInfo, source) {
		return fmt.Errorf("cannot record recovery layout: %s does not match the pool device", device)
	}
	uuid, err := system.readDeviceUUID(device)
	if err != nil {
		return err
	}
	saved := &poolMountSnapshot{
		sourcePath: sourcePath, targetPath: targetPath, hostPath: hostPath, source: *source,
		device: device, deviceUUID: uuid, enforceQuota: quotaEnabled(source),
	}
	for i, info := range hostPlan {
		// Keep the backing pool and its nested mounts, not workload PVC mounts
		// under other kubelet pod paths that may legitimately be deleted later.
		if i == 0 || info.Root == source.Root || pathWithin(info.Mountpoint, targetPath) {
			entry := *info
			saved.hostMounts = append(saved.hostMounts, &entry)
			log.V(3).Info("Recording expected host mount", "target", info.Mountpoint, "filesystemRoot", info.Root, "options", info.Options)
		}
	}
	for _, info := range containerPlan {
		entry := *info
		saved.containerMounts = append(saved.containerMounts, &entry)
		log.V(3).Info("Recording expected container mount", "target", info.Mountpoint, "filesystemRoot", info.Root, "options", info.Options)
	}
	// Publish the snapshot only after every validation above has succeeded.
	r.saved = saved
	r.system = system
	r.pendingSetup = make(map[string]bool)
	log.Info("Saved pool mount layout for recovery", "device", device, "filesystemUUID", uuid, "hostMountCount", len(saved.hostMounts), "containerMountCount", len(saved.containerMounts))
	return nil
}

// Block pools have no container filesystem bind at /source. Record their host
// filesystem after the mounter creates it, using the raw device's identity.
func rememberBlockPoolMounts(sourcePath, targetPath, hostPath string) error {
	var deviceInfo unix.Stat_t
	if err := unix.Stat(sourcePath, &deviceInfo); err != nil {
		return err
	}
	mounts, err := readQuotaMounts(filepath.Join(hostPath, "proc/1/mountinfo"))
	if err != nil {
		return err
	}
	for _, info := range mounts {
		if info.Mountpoint != targetPath {
			continue
		}
		if !matchesPoolDevice(&deviceInfo, info) {
			return fmt.Errorf("pool target %s does not match its raw block device", targetPath)
		}
		if _, err := recoveryMountPresent(mounts, info); err != nil {
			return err
		}
		return rememberedPoolMounts.capture(sourcePath, targetPath, hostPath, info, false, filesystemQuotaSystem(hostPath, info))
	}
	return fmt.Errorf("block pool target %s is not mounted", targetPath)
}

// Initial capture may precede quota setup. Once setup succeeds, future recovery
// must use quota-enabled options rather than the original no-quota settings.
func (r *poolMountRecovery) enableQuota() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved != nil && !r.saved.enforceQuota {
		r.saved.enforceQuota = true
		log.Info("Updated saved pool mount options to enforce project quotas", "target", r.saved.targetPath)
	}
}

// The final pool bind may not exist when the initial layout is captured.
// Add it after creation without replacing or forgetting the earlier records.
func (r *poolMountRecovery) rememberTarget() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved == nil {
		return nil
	}
	saved := r.saved
	mounts, err := r.system.readMounts(filepath.Join(saved.hostPath, "proc/1/mountinfo"))
	if err != nil {
		return err
	}
	expected := saved.source
	expected.Mountpoint = saved.targetPath
	present, err := recoveryMountPresent(mounts, &expected)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("pool target %s is not mounted; cannot add it to the recovery record", saved.targetPath)
	}
	for _, info := range saved.hostMounts {
		if info.Mountpoint == saved.targetPath {
			return nil
		}
	}
	for _, info := range mounts {
		if info.Mountpoint == saved.targetPath {
			entry := *info
			saved.hostMounts = append(saved.hostMounts, &entry)
		}
	}
	// Preserve the filesystem root at index zero; put the remaining binds parent first.
	sort.Slice(saved.hostMounts[1:], func(i, j int) bool {
		return len(saved.hostMounts[i+1].Mountpoint) < len(saved.hostMounts[j+1].Mountpoint)
	})
	log.Info("Added pool target to mount recovery record", "target", saved.targetPath, "hostMountCount", len(saved.hostMounts))
	return nil
}

// Different device names can refer to the same filesystem, so compare the
// filesystem number, exposed directory, and filesystem type instead of Source.
func samePoolMount(expected, actual *mount.Info) bool {
	return expected.Major == actual.Major && expected.Minor == actual.Minor && expected.Root == actual.Root && expected.FSType == actual.FSType
}

// An exact mountpoint is required: a parent filesystem covering the path does
// not mean the saved bind still exists. Never stack over a replacement mount.
func recoveryMountPresent(mounts []*mount.Info, expected *mount.Info) (bool, error) {
	count := 0
	for _, actual := range mounts {
		if actual.Mountpoint != expected.Mountpoint {
			continue
		}
		if !samePoolMount(expected, actual) {
			return false, fmt.Errorf("refusing to recover %s: a different mount occupies the path", expected.Mountpoint)
		}
		count++
	}
	if count > 1 {
		return false, fmt.Errorf("refusing to recover stacked mounts at %s", expected.Mountpoint)
	}
	if count == 1 {
		return true, nil
	}
	for _, actual := range mounts {
		if pathWithin(actual.Mountpoint, expected.Mountpoint) {
			return false, fmt.Errorf("refusing to cover nested mount %s while recovering %s", actual.Mountpoint, expected.Mountpoint)
		}
	}
	return false, nil
}

// Reconcile host mounts first, then the container source. The same device lock
// used by quota setup prevents two mounters from changing this filesystem together.
func (r *poolMountRecovery) recover() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved == nil {
		return nil
	}
	saved, system := r.saved, r.system
	unlock, err := system.lock()
	if err != nil {
		return err
	}
	defer unlock()
	// Host mounts supply the directories used to rebuild the container binds.
	for _, info := range saved.hostMounts {
		if err := r.recoverMount(info, false); err != nil {
			return fmt.Errorf("failed to recover host mount %s; dependent mounts will wait: %w", info.Mountpoint, err)
		}
	}
	for _, info := range saved.containerMounts {
		if err := r.recoverMount(info, true); err != nil {
			return fmt.Errorf("failed to recover container mount %s; dependent mounts will wait: %w", info.Mountpoint, err)
		}
	}
	return nil
}

// Recover one entry in four steps: inspect it, verify the device, create the
// missing mount, then restore its settings and check the result.
func (r *poolMountRecovery) recoverMount(info *mount.Info, inContainer bool) error {
	table := filepath.Join(r.saved.hostPath, "proc/1/mountinfo")
	namespace := "host"
	run := r.system.runHost
	if inContainer {
		table = "/proc/self/mountinfo"
		namespace = "container"
		run = r.system.runLocal
	}
	filesystemMount := !inContainer && info == r.saved.hostMounts[0]
	requireQuota := filesystemMount && r.saved.enforceQuota
	present, err := r.inspectRecoveryMount(table, info, requireQuota)
	if err != nil {
		return err
	}
	key := namespace + ":" + info.Mountpoint
	if present && !r.pendingSetup[key] {
		return nil // Healthy mounts need no commands or normal-level log messages.
	}
	if err := r.verifyRecoveryDevice(); err != nil {
		return err
	}
	if !present {
		if err := r.createRecoveryMount(info, inContainer, run); err != nil {
			return err
		}
	} else {
		log.Info("Retrying pool mount settings after partial recovery", "namespace", namespace, "target", info.Mountpoint)
	}

	// A previous bind may exist even though restoring its options failed.
	// Finish that work without placing another mount on top of it.
	if !filesystemMount {
		if err := run("/usr/bin/mount", "-o", "remount,bind,"+info.Options, info.Mountpoint); err != nil {
			return fmt.Errorf("failed to restore bind options for %s: %w", info.Mountpoint, err)
		}
	}
	if err := restoreQuotaPropagation(info, run); err != nil {
		return fmt.Errorf("failed to restore propagation for %s: %w", info.Mountpoint, err)
	}
	present, err = r.inspectRecoveryMount(table, info, requireQuota)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("mount %s is still missing after recovery", info.Mountpoint)
	}
	delete(r.pendingSetup, key)
	log.Info("Recovered pool mount", "namespace", namespace, "target", info.Mountpoint)
	return nil
}

// Read the actual namespace after each mount attempt. A successful command exit
// alone cannot prove that the expected filesystem and quota flags are present.
func (r *poolMountRecovery) inspectRecoveryMount(table string, expected *mount.Info, requireQuota bool) (bool, error) {
	mounts, err := r.system.readMounts(table)
	if err != nil {
		return false, fmt.Errorf("failed to read mount table %s: %w", table, err)
	}
	present, err := recoveryMountPresent(mounts, expected)
	if err != nil || !present {
		return present, err
	}
	if requireQuota {
		for _, actual := range mounts {
			if actual.Mountpoint == expected.Mountpoint && !quotaEnabled(actual) {
				return false, fmt.Errorf("project quotas are missing on pool filesystem %s", expected.Mountpoint)
			}
		}
	}
	return true, nil
}

// Device numbers can be reused after detach. The saved UUID must also match
// before recovery is allowed to mount the device again.
func (r *poolMountRecovery) verifyRecoveryDevice() error {
	saved := r.saved
	var deviceInfo unix.Stat_t
	if err := r.system.stat(filepath.Join(saved.hostPath, saved.device), &deviceInfo); err != nil {
		return fmt.Errorf("failed to inspect recovery device %s: %w", saved.device, err)
	}
	if !matchesPoolDevice(&deviceInfo, &saved.source) {
		return fmt.Errorf("refusing mount recovery: pool device %s changed", saved.device)
	}
	uuid, err := r.system.readDeviceUUID(saved.device)
	if err != nil {
		return err
	}
	if uuid != saved.deviceUUID {
		return fmt.Errorf("refusing mount recovery: filesystem UUID changed for %s", saved.device)
	}
	log.V(3).Info("Verified pool device for recovery", "device", saved.device, "filesystemUUID", uuid)
	return nil
}

// Create the filesystem mount from its device, or a bind from the restored pool.
// Missing directories are left alone because kubelet may have deliberately removed them.
func (r *poolMountRecovery) createRecoveryMount(info *mount.Info, inContainer bool, run func(string, ...string) error) error {
	saved := r.saved
	target := filepath.Join(saved.hostPath, info.Mountpoint)
	namespace := "host"
	if inContainer {
		target = info.Mountpoint
		namespace = "container"
	}
	var targetInfo unix.Stat_t
	if err := r.system.stat(target, &targetInfo); err != nil {
		return fmt.Errorf("recovery target %s is unavailable: %w", target, err)
	}
	if targetInfo.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("recovery target %s is not a directory", target)
	}
	log.Info("Recovering missing pool mount", "namespace", namespace, "target", info.Mountpoint, "filesystemUUID", saved.deviceUUID)
	r.pendingSetup[namespace+":"+info.Mountpoint] = true
	root := saved.hostMounts[0]
	if !inContainer && info == root {
		return restoreQuotaMount(info, root, saved.device, saved.enforceQuota, run)
	}

	// Root names a directory inside the pool filesystem, such as / or /child.
	bindSource := filepath.Join(root.Mountpoint, strings.TrimPrefix(info.Root, "/"))
	statPath := filepath.Join(saved.hostPath, bindSource)
	if inContainer {
		bindSource = statPath // Container commands reach node directories through /host.
	}
	var sourceInfo unix.Stat_t
	if err := r.system.stat(statPath, &sourceInfo); err != nil {
		return fmt.Errorf("failed to inspect recovery bind source %s: %w", statPath, err)
	}
	if int(unix.Major(uint64(sourceInfo.Dev))) != saved.source.Major || int(unix.Minor(uint64(sourceInfo.Dev))) != saved.source.Minor {
		return fmt.Errorf("refusing to recover bind from unavailable pool %s", bindSource)
	}
	log.Info("Recreating pool bind mount", "namespace", namespace, "source", bindSource, "target", info.Mountpoint)
	return run("/usr/bin/mount", "--bind", bindSource, info.Mountpoint)
}

// Rdev identifies the block device itself; Dev would identify the filesystem
// containing the device node, which is usually /dev and is not the pool.
func matchesPoolDevice(deviceInfo *unix.Stat_t, source *mount.Info) bool {
	return deviceInfo.Mode&unix.S_IFMT == unix.S_IFBLK &&
		int(unix.Major(uint64(deviceInfo.Rdev))) == source.Major &&
		int(unix.Minor(uint64(deviceInfo.Rdev))) == source.Minor
}
