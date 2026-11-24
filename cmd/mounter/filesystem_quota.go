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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	mount "github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

func readQuotaMounts(path string) ([]*mount.Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return mount.GetMountsFromReader(f, nil)
}

func quotaEnabled(info *mount.Info) bool {
	options := strings.Split(info.VFSOptions+","+info.Options, ",")
	enabled := false
	for _, option := range options {
		switch option {
		case "prjquota", "pquota":
			enabled = true
		case "noquota", "pqnoenforce":
			return false
		}
	}
	return enabled
}

func pathWithin(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// quotaMountPlan lists the filesystem root first, then bind mounts in parent-first order.
func quotaMountPlan(hostMounts []*mount.Info, sourceMount *mount.Info) ([]*mount.Info, error) {
	// Collect the pool filesystem's mounts, then order them for unmounting and restoration.
	var hostMountPlan []*mount.Info
	// Track mountpoint paths so duplicate entries (stacked mounts) are rejected.
	seen := map[string]bool{}
	for _, info := range hostMounts {
		if info.Major != sourceMount.Major || info.Minor != sourceMount.Minor {
			continue
		}
		if info.Mountpoint == "/" {
			return nil, fmt.Errorf("refusing to change quotas on the node root filesystem")
		}
		if strings.Contains(","+info.Options+",", ",ro,") {
			return nil, fmt.Errorf("read-only pool mount %s requires manual quota preparation", info.Mountpoint)
		}
		if seen[info.Mountpoint] {
			return nil, fmt.Errorf("stacked mounts at %s require manual quota preparation", info.Mountpoint)
		}
		seen[info.Mountpoint] = true
		hostMountPlan = append(hostMountPlan, info)
	}
	// Never detach an unrelated filesystem nested inside this pool.
	for _, info := range hostMounts {
		for _, pool := range hostMountPlan {
			if info.Major == sourceMount.Major && info.Minor == sourceMount.Minor {
				continue
			}
			if pathWithin(info.Mountpoint, pool.Mountpoint) {
				return nil, fmt.Errorf("unrelated mount %s is nested under pool mount %s", info.Mountpoint, pool.Mountpoint)
			}
		}
	}
	// Parent paths are shorter than their children, so restore them first.
	// Unmounting uses the reverse order; the string comparison breaks length ties.
	sort.Slice(hostMountPlan, func(i, j int) bool {
		if len(hostMountPlan[i].Mountpoint) != len(hostMountPlan[j].Mountpoint) {
			return len(hostMountPlan[i].Mountpoint) < len(hostMountPlan[j].Mountpoint)
		}
		return hostMountPlan[i].Mountpoint < hostMountPlan[j].Mountpoint
	})
	rootIndex := -1
	for i, candidate := range hostMountPlan {
		// Root == "/" exposes the whole backing filesystem, rather than a subdirectory.
		// It does not mean the mountpoint is the node's root directory.
		if candidate.Root != "/" {
			continue
		}
		nested := false
		for j, other := range hostMountPlan {
			// The base mount must be accessible without restoring another pool bind first.
			if i != j && pathWithin(candidate.Mountpoint, other.Mountpoint) {
				nested = true
			}
		}
		// Choose the first full filesystem mount that is not inside another pool mount.
		if !nested {
			rootIndex = i
			break
		}
	}
	// Without a full filesystem mount, we cannot rebuild all binds from their Root paths.
	if rootIndex == -1 {
		return nil, fmt.Errorf("cannot find an independent full filesystem mount for device %d:%d", sourceMount.Major, sourceMount.Minor)
	}
	root := hostMountPlan[rootIndex]
	// Shift the earlier entries right to put the filesystem mount first.
	// For example: [bindA, root, bindB] becomes [root, bindA, bindB].
	copy(hostMountPlan[1:rootIndex+1], hostMountPlan[:rootIndex])
	hostMountPlan[0] = root
	return hostMountPlan, nil
}

func runQuotaCommand(command string, args ...string) error {
	log.V(3).Info("Running filesystem quota command", "command", command, "args", args)
	out, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		log.Error(err, "Failed to run filesystem quota command", "command", command, "args", args, "output", string(out))
		return fmt.Errorf("failed to run %s %q: %w (output: %q)", command, args, err, out)
	}
	log.V(3).Info("Filesystem quota command completed", "command", command, "args", args, "output", string(out))
	return nil
}

// Commands enter the host mount namespace; chroot alone does not change namespaces.
func hostQuotaCommand(hostPath string, run func(string, ...string) error) func(string, ...string) error {
	return func(command string, args ...string) error {
		// Use the target process's root mount, not the container's /host bind.
		// nsenter opens root and working-directory paths before switching namespaces.
		hostRoot := filepath.Join(hostPath, "proc/1/root")
		prefix := []string{"--mount=" + filepath.Join(hostPath, "proc/1/ns/mnt"), "--root=" + hostRoot, "--wd=" + hostRoot, "--", command}
		return run("/usr/bin/nsenter", append(prefix, args...)...)
	}
}

func quotaMountOptions(info *mount.Info, enable bool) string {
	var options []string
	seen := map[string]bool{}
	for _, option := range strings.Split(info.Options+","+info.VFSOptions, ",") {
		if enable && (option == "noquota" || option == "pqnoenforce" || option == "pquota" || option == "prjquota") {
			continue
		}
		if option != "" && !seen[option] {
			seen[option] = true
			options = append(options, option)
		}
	}
	if enable {
		options = append(options, "prjquota")
	}
	return strings.Join(options, ",")
}

func restoreQuotaMount(info, root *mount.Info, device string, enable bool, run func(string, ...string) error) error {
	if info == root {
		options := quotaMountOptions(info, enable)
		log.Info("Restoring pool filesystem mount", "device", device, "target", info.Mountpoint, "filesystem", info.FSType, "options", options, "enableQuota", enable)
		return run("/usr/bin/mount", "-t", info.FSType, "-o", options, device, info.Mountpoint)
	}
	// Root is the directory inside the filesystem, not a path on the node.
	source := filepath.Join(root.Mountpoint, strings.TrimPrefix(info.Root, "/"))
	log.Info("Restoring pool bind mount", "source", source, "target", info.Mountpoint, "options", info.Options)
	if err := run("/usr/bin/mount", "--bind", source, info.Mountpoint); err != nil {
		return err
	}
	return run("/usr/bin/mount", "-o", "remount,bind,"+info.Options, info.Mountpoint)
}

// Unmount children first and return only the mounts actually removed, even on failure.
// The caller must restore this list in reverse order before returning.
func unmountQuotaMounts(mountPlan []*mount.Info, namespace string, run func(string, ...string) error) ([]*mount.Info, error) {
	var unmountedMounts []*mount.Info
	for i := len(mountPlan) - 1; i >= 0; i-- {
		info := mountPlan[i]
		log.Info("Unmounting pool mount", "namespace", namespace, "target", info.Mountpoint, "filesystemRoot", info.Root)
		if err := run("/usr/bin/umount", info.Mountpoint); err != nil {
			return unmountedMounts, err
		}
		unmountedMounts = append(unmountedMounts, info)
		log.Info("Unmounted pool mount", "namespace", namespace, "target", info.Mountpoint)
	}
	return unmountedMounts, nil
}

// Restore the filesystem before its binds. If a parent cannot be restored, leave
// its children alone so they cannot accidentally expose directories on the node root.
func restoreHostQuotaMounts(unmountedMounts []*mount.Info, filesystemMount *mount.Info, device string, enableQuota bool, run func(string, ...string) error) (err error) {
	log.Info("Restoring detached host pool mounts", "device", device, "mountCount", len(unmountedMounts), "enableQuota", enableQuota)
	var failedPaths []string
	for i := len(unmountedMounts) - 1; i >= 0; i-- {
		info := unmountedMounts[i]
		blocked := false
		for _, failed := range failedPaths {
			if failed == filesystemMount.Mountpoint || pathWithin(info.Mountpoint, failed) {
				blocked = true
			}
		}
		if blocked {
			log.Info("Skipping mount restoration because its filesystem or parent failed to restore", "device", device, "target", info.Mountpoint, "failedPaths", failedPaths)
			continue
		}
		if restoreErr := restoreQuotaMount(info, filesystemMount, device, enableQuota, run); restoreErr != nil {
			log.Error(restoreErr, "Failed to restore pool mount", "device", device, "target", info.Mountpoint, "enableQuota", enableQuota)
			err = errors.Join(err, fmt.Errorf("failed to restore mount %s: %w", info.Mountpoint, restoreErr))
			// If the quota mount fails, try the saved options to restore availability.
			if info != filesystemMount || !enableQuota || restoreQuotaMount(info, filesystemMount, device, false, run) != nil {
				failedPaths = append(failedPaths, info.Mountpoint)
				continue
			}
			log.Info("Restored pool filesystem using original options after quota mount failed", "device", device, "target", info.Mountpoint)
		}
		if propagationErr := restoreQuotaPropagation(info, run); propagationErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to restore mount propagation for %s: %w", info.Mountpoint, propagationErr))
		}
	}
	return err
}

// withQuotaMountsDetached always tries to restore mounts, including after partial failure.
func withQuotaMountsDetached(hostMountPlan []*mount.Info, device string, run func(string, ...string) error, prepare func() error) (err error) {
	unmountedMounts, unmountErr := unmountQuotaMounts(hostMountPlan, "host", run)
	enableQuota := false
	defer func() {
		err = errors.Join(err, restoreHostQuotaMounts(unmountedMounts, hostMountPlan[0], device, enableQuota, run))
	}()
	if unmountErr != nil {
		return unmountErr
	}
	log.Info("Preparing project quotas with host pool mounts detached", "device", device)
	if err := prepare(); err != nil {
		log.Error(err, "Failed to prepare project quotas; restoring saved mounts", "device", device)
		return err
	}
	log.Info("Project quota preparation completed; restoring mounts with prjquota", "device", device)
	enableQuota = true
	return nil
}

// Check every process namespace: an invisible bind still keeps the filesystem mounted.
func quotaDeviceUnmounted(hostPath string, source *mount.Info) error {
	paths, err := filepath.Glob(filepath.Join(hostPath, "proc/[0-9]*/mountinfo"))
	if err != nil || len(paths) == 0 {
		return fmt.Errorf("cannot inspect host mount namespaces: %v", err)
	}
	log.V(3).Info("Checking remaining pool mounts across process namespaces", "majorMinor", fmt.Sprintf("%d:%d", source.Major, source.Minor), "processCount", len(paths))
	for _, path := range paths {
		infos, err := readQuotaMounts(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to inspect %s: %w", path, err)
		}
		for _, info := range infos {
			if info.Major == source.Major && info.Minor == source.Minor {
				log.Info("Pool device still has a mount reference; refusing offline quota changes", "mountinfoPath", path, "target", info.Mountpoint, "device", info.Source, "majorMinor", fmt.Sprintf("%d:%d", source.Major, source.Minor))
				return fmt.Errorf("device %d:%d is still mounted at %s in %s; coordinate quota preparation with the backing CSI driver", source.Major, source.Minor, info.Mountpoint, path)
			}
		}
	}
	return nil
}

// Isolate only the parents of mounts we will detach, without recursively changing
// /host or its shared submounts. The container still receives upstream mount changes.
func isolateQuotaUnmountParents(all, plan []*mount.Info, hostPath string, system filesystemQuotaOps) error {
	byID := make(map[int]*mount.Info, len(all))
	for _, info := range all {
		byID[info.ID] = info
	}
	parents := map[int]*mount.Info{}
	for _, info := range plan {
		parent, ok := byID[info.Parent]
		if !ok {
			return fmt.Errorf("cannot find parent mount %d for container source %s", info.Parent, info.Mountpoint)
		}
		if strings.Contains(parent.Optional, "shared:") {
			if pathWithin(parent.Mountpoint, hostPath) {
				return fmt.Errorf("refusing to change propagation of host-root mount %s", parent.Mountpoint)
			}
			parents[parent.ID] = parent
		}
	}
	if len(parents) == 0 {
		return nil
	}

	// Never change the node's root propagation if the process is in the host namespace.
	var currentNS, hostNS unix.Stat_t
	if err := system.stat("/proc/self/ns/mnt", &currentNS); err != nil {
		return fmt.Errorf("failed to identify container mount namespace: %w", err)
	}
	if err := system.stat(filepath.Join(hostPath, "proc/1/ns/mnt"), &hostNS); err != nil {
		return fmt.Errorf("failed to identify host mount namespace: %w", err)
	}
	if currentNS.Dev == hostNS.Dev && currentNS.Ino == hostNS.Ino {
		return fmt.Errorf("refusing to isolate mount parents in the host mount namespace")
	}

	ordered := make([]*mount.Info, 0, len(parents))
	for _, parent := range parents {
		ordered = append(ordered, parent)
	}
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i].Mountpoint) < len(ordered[j].Mountpoint) })
	for _, parent := range ordered {
		log.Info("Configuring container mount parent to receive upstream changes before quota setup", "target", parent.Mountpoint, "originalPropagation", parent.Optional)
		// Keep external container parents one-way for this process; do not recursively change /host.
		if err := system.runLocal("/usr/bin/mount", "--make-slave", parent.Mountpoint); err != nil {
			return fmt.Errorf("failed to isolate container mount parent %s: %w", parent.Mountpoint, err)
		}
	}

	// Every shared parent is now isolated. Keep the original records for restoration.
	return nil
}

func restoreQuotaPropagation(info *mount.Info, run func(string, ...string) error) error {
	var flags []string
	// Establish the upstream relationship before allowing the mount to share changes downstream.
	if strings.Contains(info.Optional, "master:") {
		flags = append(flags, "--make-slave")
	}
	if strings.Contains(info.Optional, "shared:") {
		flags = append(flags, "--make-shared")
	}
	if strings.Contains(info.Optional, "unbindable") {
		flags = append(flags, "--make-unbindable")
	}
	if len(flags) == 0 {
		flags = append(flags, "--make-private")
	}
	for _, flag := range flags {
		log.V(3).Info("Restoring mount propagation", "target", info.Mountpoint, "flag", flag)
		if err := run("/usr/bin/mount", flag, info.Mountpoint); err != nil {
			return err
		}
	}
	return nil
}

func lockFilesystemQuotaSetup(hostPath string, source *mount.Info) (func(), error) {
	lockDir := filepath.Join(hostPath, "run/lock/hpp-project-quota")
	if err := os.MkdirAll(lockDir, 0750); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(lockDir, fmt.Sprintf("%d-%d", source.Major, source.Minor)), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	// Serialize setup across mounter processes using the same node filesystem.
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("another mounter is preparing quotas for %d:%d: %w", source.Major, source.Minor, err)
	}
	return func() {
		unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		lock.Close()
	}, nil
}

// Keep host operations replaceable so the complete flow can be tested without mounting disks.
type filesystemQuotaOps struct {
	readMounts     func(string) ([]*mount.Info, error)
	stat           func(string, *unix.Stat_t) error
	runLocal       func(string, ...string) error
	runHost        func(string, ...string) error
	lock           func() (func(), error)
	checkUnmounted func() error
}

func enableFilesystemPoolQuota(sourcePath, hostPath string, source *mount.Info) error {
	return enableFilesystemPoolQuotaWithOps(sourcePath, hostPath, source, filesystemQuotaOps{
		readMounts:     readQuotaMounts,
		stat:           unix.Stat,
		runLocal:       runQuotaCommand,
		runHost:        hostQuotaCommand(hostPath, runQuotaCommand),
		lock:           func() (func(), error) { return lockFilesystemQuotaSetup(hostPath, source) },
		checkUnmounted: func() error { return quotaDeviceUnmounted(hostPath, source) },
	})
}

// Check device identity, host tools, and namespace visibility before removing any mounts.
func checkQuotaPrerequisites(hostPath, device string, sourceMount *mount.Info, hostMountPlan []*mount.Info, system filesystemQuotaOps) (uint64, error) {
	var deviceInfo unix.Stat_t
	if err := system.stat(filepath.Join(hostPath, device), &deviceInfo); err != nil {
		return 0, fmt.Errorf("failed to stat pool device %s: %w", device, err)
	}
	if deviceInfo.Mode&unix.S_IFMT != unix.S_IFBLK || int(unix.Major(uint64(deviceInfo.Rdev))) != sourceMount.Major || int(unix.Minor(uint64(deviceInfo.Rdev))) != sourceMount.Minor {
		return 0, fmt.Errorf("mount source %s does not match pool block device", device)
	}
	commands := []string{"/usr/bin/mount", "/usr/bin/umount", "/usr/bin/findmnt"}
	if sourceMount.FSType == "ext4" {
		commands = append(commands, "/usr/sbin/tune2fs")
	}
	log.V(3).Info("Checking quota tools in host namespace", "commands", commands)
	for _, command := range commands {
		if err := system.runHost("/usr/bin/test", "-x", command); err != nil {
			return 0, err
		}
	}
	// The host command must see the same mountpoints as the mount table we saved.
	for _, info := range hostMountPlan {
		if err := system.runHost("/usr/bin/findmnt", "--mountpoint", info.Mountpoint, "--noheadings", "--output", "TARGET"); err != nil {
			return 0, fmt.Errorf("host command cannot see planned mount %s: %w", info.Mountpoint, err)
		}
	}
	return uint64(deviceInfo.Rdev), nil
}

// Save the container source and nested binds, then prevent their unmounts from
// propagating back to the host. No mounts are removed here.
func prepareContainerQuotaUnmounts(sourcePath, hostPath string, sourceMount *mount.Info, system filesystemQuotaOps) ([]*mount.Info, error) {
	containerMounts, err := system.readMounts("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	var containerMountPlan []*mount.Info
	seenPaths := map[string]bool{}
	for _, info := range containerMounts {
		if !pathWithin(info.Mountpoint, sourcePath) {
			continue
		}
		if info.Major != sourceMount.Major || info.Minor != sourceMount.Minor {
			return nil, fmt.Errorf("unrelated mount %s is nested under %s", info.Mountpoint, sourcePath)
		}
		if seenPaths[info.Mountpoint] {
			return nil, fmt.Errorf("stacked container mounts at %s require manual quota preparation", info.Mountpoint)
		}
		seenPaths[info.Mountpoint] = true
		containerMountPlan = append(containerMountPlan, info)
	}
	if !seenPaths[filepath.Clean(sourcePath)] {
		return nil, fmt.Errorf("cannot find mounter source mount %s", sourcePath)
	}
	// Restore parents first; unmountQuotaMounts removes them in the reverse order.
	sort.Slice(containerMountPlan, func(i, j int) bool {
		return len(containerMountPlan[i].Mountpoint) < len(containerMountPlan[j].Mountpoint)
	})
	if err := isolateQuotaUnmountParents(containerMounts, containerMountPlan, hostPath, system); err != nil {
		return nil, err
	}
	return containerMountPlan, nil
}

// Rebuild only the container binds we removed, after the host filesystem is back.
func restoreContainerQuotaMounts(unmountedMounts []*mount.Info, hostFilesystemPath string, sourceMount *mount.Info, system filesystemQuotaOps) (err error) {
	log.Info("Restoring detached container source mounts", "mountCount", len(unmountedMounts))
	for i := len(unmountedMounts) - 1; i >= 0; i-- {
		info := unmountedMounts[i]
		// Root is this bind's directory within the pool, e.g. / or /some/subdirectory.
		bindSource := filepath.Join(hostFilesystemPath, strings.TrimPrefix(info.Root, "/"))
		var current unix.Stat_t
		// Never bind the empty directory underneath a host filesystem that failed to return.
		if statErr := system.stat(bindSource, &current); statErr != nil || int(unix.Major(uint64(current.Dev))) != sourceMount.Major || int(unix.Minor(uint64(current.Dev))) != sourceMount.Minor {
			err = errors.Join(err, fmt.Errorf("refusing to restore container bind from unavailable pool %s (stat: %v)", bindSource, statErr))
			continue
		}
		log.Info("Restoring container source bind mount", "source", bindSource, "target", info.Mountpoint, "options", info.Options)
		if restoreErr := system.runLocal("/usr/bin/mount", "--bind", bindSource, info.Mountpoint); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to restore container source %s: %w", info.Mountpoint, restoreErr))
			continue
		}
		if restoreErr := system.runLocal("/usr/bin/mount", "-o", "remount,bind,"+info.Options, info.Mountpoint); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to restore container mount options for %s: %w", info.Mountpoint, restoreErr))
		}
		if restoreErr := restoreQuotaPropagation(info, system.runLocal); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to restore container propagation for %s: %w", info.Mountpoint, restoreErr))
		}
	}
	return err
}

// Enable filesystem features only after every namespace has released the device.
func prepareQuotaDevice(hostPath, device string, sourceMount *mount.Info, expectedDevice uint64, system filesystemQuotaOps) error {
	if err := system.checkUnmounted(); err != nil {
		return err
	}
	// Recheck after unmounting: a disappeared or reused path must not target another disk.
	var current unix.Stat_t
	if statErr := system.stat(filepath.Join(hostPath, device), &current); statErr != nil || uint64(current.Rdev) != expectedDevice || current.Mode&unix.S_IFMT != unix.S_IFBLK {
		return fmt.Errorf("pool device %s changed while unmounting (stat: %v)", device, statErr)
	}
	if sourceMount.FSType == "ext4" {
		log.Info("Enabling ext4 project and quota features on unmounted device", "device", device)
		return system.runHost("/usr/sbin/tune2fs", "-O", "project,quota", "-Q", "prjquota", device)
	}
	// XFS initializes quota accounting on the next full mount with prjquota.
	log.Info("XFS device is ready to mount with project quota accounting", "device", device)
	return nil
}

// Reread the host mount table to confirm the restored filesystem enforces project quotas.
func verifyQuotaMount(hostPath string, filesystemMount *mount.Info, system filesystemQuotaOps) error {
	log.Info("Verifying project quota mount options", "device", filesystemMount.Source, "target", filesystemMount.Mountpoint)
	hostMounts, err := system.readMounts(filepath.Join(hostPath, "proc/1/mountinfo"))
	if err != nil {
		return err
	}
	for _, info := range hostMounts {
		if info.Mountpoint == filesystemMount.Mountpoint && quotaEnabled(info) {
			return nil
		}
	}
	return fmt.Errorf("project quotas are missing after mounting %s", filesystemMount.Source)
}

// Coordinate setup in order: inspect, check, detach, prepare, restore, and verify.
// Deferred restoration also runs when any step fails partway through.
func enableFilesystemPoolQuotaWithOps(sourcePath, hostPath string, sourceMount *mount.Info, system filesystemQuotaOps) (err error) {
	logger := log.WithValues("sourcePath", sourcePath, "hostPath", hostPath, "filesystem", sourceMount.FSType, "majorMinor", fmt.Sprintf("%d:%d", sourceMount.Major, sourceMount.Minor))
	defer func() {
		if err != nil {
			logger.Error(err, "Filesystem pool quota setup failed")
		}
	}()
	logger.V(3).Info("Checking filesystem pool project quotas", "options", sourceMount.Options, "superOptions", sourceMount.VFSOptions)
	if sourceMount.FSType != "ext4" && sourceMount.FSType != "xfs" {
		logger.V(3).Info("Skipping quota setup for unsupported filesystem")
		return nil
	}
	if !filepath.IsAbs(sourcePath) || filepath.Clean(sourcePath) == "/" {
		return fmt.Errorf("invalid pool source mount %q", sourcePath)
	}
	unlock, err := system.lock()
	if err != nil {
		return err
	}
	defer unlock()

	// Inspect the host first: pools on the node root are skipped, as are enabled quotas.
	hostMounts, err := system.readMounts(filepath.Join(hostPath, "proc/1/mountinfo"))
	if err != nil {
		return fmt.Errorf("failed to inspect host mounts: %w", err)
	}
	for _, info := range hostMounts {
		if info.Mountpoint == "/" && info.Major == sourceMount.Major && info.Minor == sourceMount.Minor {
			logger.Info("Skipping project quota setup for pool on node root filesystem")
			return nil
		}
	}
	if quotaEnabled(sourceMount) {
		logger.V(3).Info("Project quotas are already enabled; skipping mount changes")
		return nil
	}
	logger.Info("Starting filesystem pool project quota setup")
	hostMountPlan, err := quotaMountPlan(hostMounts, sourceMount)
	if err != nil {
		return err
	}
	filesystemMount := hostMountPlan[0]
	// RHCOS may prefix the source device with its host-root path.
	device := strings.TrimPrefix(filesystemMount.Source, rhcosPrefix)
	logger.Info("Planned host pool mounts for quota setup", "device", device, "filesystemMount", filesystemMount.Mountpoint, "mountCount", len(hostMountPlan))
	for i, info := range hostMountPlan {
		logger.Info("Saved host pool mount", "restoreOrder", i, "target", info.Mountpoint, "filesystemRoot", info.Root, "options", info.Options, "superOptions", info.VFSOptions, "propagation", info.Optional)
	}
	deviceNumber, err := checkQuotaPrerequisites(hostPath, device, sourceMount, hostMountPlan, system)
	if err != nil {
		return err
	}
	containerMountPlan, err := prepareContainerQuotaUnmounts(sourcePath, hostPath, sourceMount, system)
	if err != nil {
		return err
	}

	// Release container binds first. Always restore the successfully removed mounts,
	// even when another mount in this same list refuses to unmount.
	unmountedContainerMounts, unmountErr := unmountQuotaMounts(containerMountPlan, "container", system.runLocal)
	defer func() {
		hostFilesystemPath := filepath.Join(hostPath, filesystemMount.Mountpoint)
		err = errors.Join(err, restoreContainerQuotaMounts(unmountedContainerMounts, hostFilesystemPath, sourceMount, system))
	}()
	if unmountErr != nil {
		return unmountErr
	}
	// This call restores host mounts before returning, so container restoration can use them.
	if err := withQuotaMountsDetached(hostMountPlan, device, system.runHost, func() error {
		return prepareQuotaDevice(hostPath, device, sourceMount, deviceNumber, system)
	}); err != nil {
		return err
	}
	if err := verifyQuotaMount(hostPath, filesystemMount, system); err != nil {
		return err
	}
	logger.Info("Enabled filesystem pool project quotas", "device", device)
	return nil
}
