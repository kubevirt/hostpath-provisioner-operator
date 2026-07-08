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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	mount "github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	rhcosPrefix = "/ostree/deploy/rhcos"
)

var (
	log = logf.Log.WithName("mounter")

	sourceRgx = regexp.MustCompile(`\[(.+)\]`)

	findMntByVolume = func(volumeName string) ([]byte, error) {
		return exec.Command("/usr/bin/findmnt", "-T", fmt.Sprintf("/%s", volumeName), "-J").CombinedOutput()
	}

	findMntBySource = func(source string) ([]byte, error) {
		return exec.Command("/usr/bin/findmnt", "-S", source, "-J").CombinedOutput()
	}

	bindMountCommand = func(source, target string) ([]byte, error) {
		return exec.Command("/usr/bin/mount", "-o", "bind", source, target).CombinedOutput()
	}

	mountDeviceCommand = func(source, target string) ([]byte, error) {
		return exec.Command("/usr/bin/mount", source, target).CombinedOutput()
	}

	unmountPathCommand = func(target string) ([]byte, error) {
		return exec.Command("/usr/bin/umount", target).CombinedOutput()
	}

	fsTypeCommand = func(source string) ([]byte, error) {
		return exec.Command("/usr/sbin/blkid", source, "-s", "TYPE", "-o", "value").CombinedOutput()
	}

	lsblkCommand = func(source string) ([]byte, error) {
		return exec.Command("/usr/bin/lsblk", source, "-J").CombinedOutput()
	}

	createXfs = func(source string) ([]byte, error) {
		return exec.Command("/usr/sbin/mkfs.xfs", source).CombinedOutput()
	}
)

// DeviceInfo returns device information returned by lsblk
type DeviceInfo struct {
	Name       string `json:"name"`
	Majmin     string `json:"maj:min"`
	Size       string `json:"size"`
	Type       string `json:"type"`
	Mountpoint string `json:"mountpoint"`
}

// BlockDevices is a list of DeviceInfos from the output of lsblk
type BlockDevices struct {
	Blockdevices []DeviceInfo `json:"blockdevices"`
}

// FindmntInfo contains parsed findmnt -J output.
type FindmntInfo struct {
	Target  string `json:"target"`
	Source  string `json:"source"`
	Fstype  string `json:"fstype"`
	Options string `json:"options"`
}

// FileSystems is a slice of FindmntInfo, used to parse findmnt -J output
type FileSystems struct {
	Filesystems []FindmntInfo `json:"filesystems"`
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func main() {
	var (
		sourcePath string
		targetPath string
		hostPath   string
		unmount    bool
	)
	flag.Set("logtostderr", "true")
	flag.StringVar(&sourcePath, "storagePoolPath", "/source", "path the source storagePool is mounted under")
	flag.StringVar(&targetPath, "mountPath", "/", "target path the volume should be mounted on the host")
	flag.StringVar(&hostPath, "hostPath", "/", "path of the host in container")
	flag.BoolVar(&unmount, "unmount", false, "set to have the target path unmounted")

	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	printVersion()

	if unmount {
		for {
			if !unmountPath(targetPath, hostPath) {
				time.Sleep(time.Second)
			} else {
				return
			}
		}
	} else {
		i := 0
		for {
			isBlock, err := isBlockDevice(sourcePath)
			if err != nil {
				panic(err)
			}

			if !isBlock {
				mountFileSystemVolume(sourcePath, targetPath, hostPath)
			} else {
				mountBlockVolume(sourcePath, targetPath, hostPath)
			}
			time.Sleep(time.Second)
			i++
			if i%100 == 0 {
				log.Info("Slept 100 seconds")
			}
		}
	}
}

func unmountPath(targetPath, hostPath string) bool {
	exit, err := chroot(hostPath)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := exit()
		if err != nil {
			panic(err)
		}
	}()
	beforeUnmountInfos, err := lookupFindmntInfoByVolume(targetPath)
	if err != nil {
		log.Error(err, "unable to determine volume info", "path", targetPath)
	}
	out, err := unmountPathCommand(targetPath)
	if err != nil {
		log.Error(err, "unable to determine filesystem type on device")
		log.Info("Output", "out", string(out))
	}
	afterUnmountInfos, err := lookupFindmntInfoByVolume(targetPath)
	if err != nil {
		log.Error(err, "unable to determine volume info", "path", targetPath)
	}
	return beforeUnmountInfos[0].GetSourcePath() == afterUnmountInfos[0].GetSourcePath()
}

func mountFileSystemVolume(sourcePath, targetPath, hostPath string) {
	// get mounts within the container
	mountf, err := os.Open("/proc/1/mountinfo")
	if err != nil {
		panic(err)
	}
	defer mountf.Close()

	sourceMounts, err := mount.GetMountsFromReader(mountf, mount.SingleEntryFilter(sourcePath))
	if err != nil {
		log.Error(err, "failed to get source mount", "sourcePath", sourcePath)
		return
	}
	if len(sourceMounts) == 0 {
		log.Info("no mount point entries at sourcePath", "sourcePath", sourcePath)
		return
	}

	bindMountPathOnHost(sourceMounts[0], targetPath, hostPath)
}

func mountBlockVolume(sourcePath, targetPath, hostPath string) {
	deviceInfos, err := lookupDeviceInfoByVolume(sourcePath)
	if err != nil {
		panic(err)
	}
	if len(deviceInfos) > 1 {
		log.Info("Multiple device infos found")
	} else if len(deviceInfos) == 0 {
		log.Info("No device info found")
		panic("No device infos found")
	}
	exit, err := chroot(hostPath)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := exit()
		if err != nil {
			panic(err)
		}
	}()

	// Check if filesystem exists on device
	out, err := fsTypeCommand(deviceInfos[0].GetSourceDevice())
	if err != nil {
		log.Error(err, "unable to determine filesystem type on device")
		log.Info("Output", "out", string(out))
	}
	if len(out) == 0 {
		out, err := createXfs(deviceInfos[0].GetSourceDevice())
		log.Info("Output", "out", string(out))
		if err != nil {
			panic(err)
		}
	}
	mountIfNotMounted(targetPath, hostPath, deviceInfos[0].GetSourceDevice())
}

// performs a bindmount on the host if one is needed
func bindMountPathOnHost(sourceMount *mount.Info, targetPath, hostPath string) {
	exit, err := chroot(hostPath)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := exit()
		if err != nil {
			panic(err)
		}
	}()

	// get mounts on the host
	hostMountf, err := os.Open("/proc/1/mountinfo")
	if err != nil {
		log.Error(err, "unable to open host mountinfo")
		os.Exit(1)
	}
	defer hostMountf.Close()

	major, minor := sourceMount.Major, sourceMount.Minor

	// filter out any mounts that aren't associated with the source device
	hostMounts, err := mount.GetMountsFromReader(hostMountf, func(m *mount.Info) (skip, stop bool) {
		return m.Major != major || m.Minor != minor, false
	})
	if err != nil {
		log.Error(err, "unable to read host mountinfo")
		os.Exit(1)
	}

	bindSource := ""
	for _, m := range hostMounts {
		// if a mount exists on the host with our targetPath as the mount point, no new bind mount is needed
		if m.Mountpoint == targetPath {
			return
		}

		// only consider if existing mount is a kubelet pod mount mapped to the root filesystem
		// and is not a consumer mount, indicated by the Root="/subdir"
		if bindSource == "" && strings.HasPrefix(m.Mountpoint, "/var/lib/kubelet/pods/") && m.Root == "/" {
			bindSource = m.Mountpoint
		}
	}
	if bindSource != "" {
		log.Info("Bind mounting", "source", bindSource, "target", targetPath)
		out, err := bindMountCommand(bindSource, targetPath)
		if err != nil {
			log.Error(err, "failed to bind mount")
		}
		log.Info("Output", "out", string(out))
		return
	}

	log.Info("Unable to find kubelet pod mount for filesystem", "major", major, "minor", minor)
	os.Exit(1)
}

func mountIfNotMounted(targetPath, hostPath, hostMountPath string) {
	// Check if path is already mounted
	chrootInfos, err := lookupFindmntInfoByVolume(targetPath)
	if err != nil {
		log.Error(err, "unable to determine volume info", "path", targetPath)
	}
	if len(chrootInfos) == 0 || chrootInfos[0].GetSourcePath() != hostMountPath {
		log.Info("Found mount info", "source path on host", hostMountPath)
		log.Info("Target path", "path", targetPath)
		log.Info("host path", "path", hostPath)
		_, err := os.Stat(hostMountPath)
		if os.IsNotExist(err) {
			mountInfo, err := findMntBySource(hostMountPath)
			if err != nil {
				panic(err)
			}
			infos, err := parseMntInfoJSON(mountInfo)
			if err != nil {
				panic(err)
			}
			if len(infos) == 0 {
				log.Info("No mount info found for source", "source", hostMountPath)
				os.Exit(1)
			}
			if len(infos) > 1 {
				log.Info("Multiple mount infos found, filtering for global CSI mount", "count", len(infos))
				filtered := filterGlobalMounts(infos)
				if len(filtered) != 1 {
					log.Info("Unable to determine unique global mount", "count", len(filtered))
					os.Exit(1)
				}
				infos = filtered
			}
			log.Info("Found host path by source", "path", infos[0].Target)
			hostMountPath = string(infos[0].Target)
		}
		pathInfo, err := os.Stat(hostMountPath)
		if err != nil {
			panic(err)
		}
		if pathInfo.IsDir() {
			log.Info("Bind mounting", "path", hostMountPath)
			out, err := bindMountCommand(hostMountPath, targetPath)
			if err != nil {
				log.Error(err, "failed to mount path on host.")
			}
			log.Info("Output", "out", string(out))
		} else if pathInfo.Mode()&os.ModeDevice > 0 {
			log.Info("Mounting device", "path", hostMountPath)
			// Make sure the target exists.
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				panic(err)
			}
			out, err := mountDeviceCommand(hostMountPath, targetPath)
			if err != nil {
				log.Error(err, "failed to mount device to path on host.")
			}
			log.Info("Output", "out", string(out))
		}
	}
}

func isBlockDevice(path string) (bool, error) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return pathInfo.Mode()&os.ModeDevice > 0, nil
}

func lookupDeviceInfoByVolume(volumePath string) ([]DeviceInfo, error) {
	deviceInfoJSON, err := lsblkCommand(volumePath)
	if err != nil {
		return make([]DeviceInfo, 0), err
	}
	blockDevices := BlockDevices{}
	if err := json.Unmarshal(deviceInfoJSON, &blockDevices); err != nil {
		return make([]DeviceInfo, 0), err
	}
	return blockDevices.Blockdevices, nil
}

// lookupFindmntInfoByVolume looks up the find mount information based on the path passed in.
func lookupFindmntInfoByVolume(volumePath string) ([]FindmntInfo, error) {
	mntInfoJSON, err := findMntByVolume(volumePath)
	if err != nil {
		return make([]FindmntInfo, 0), err
	}
	return parseMntInfoJSON(mntInfoJSON)
}

func parseMntInfoJSON(mntInfoJSON []byte) ([]FindmntInfo, error) {
	mntinfo := FileSystems{}
	if err := json.Unmarshal(mntInfoJSON, &mntinfo); err != nil {
		return mntinfo.Filesystems, errors.Wrapf(err, "unable to unmarshal [%v]", mntInfoJSON)
	}
	return mntinfo.Filesystems, nil
}

// GetSourcePath returns the source part of the source field. The source field format is /dev/device[/path/on/device]
func (f *FindmntInfo) GetSourcePath() string {
	match := sourceRgx.FindStringSubmatch(f.Source)
	if len(match) != 2 {
		return strings.TrimPrefix(f.Source, rhcosPrefix)
	}
	return strings.TrimPrefix(match[1], rhcosPrefix)
}

// GetOptions returns a split list of all the mount options.
func (f *FindmntInfo) GetOptions() []string {
	return strings.Split(f.Options, ",")
}

// GetSourceDevice returns the path to the device /dev/<device>
func (b *DeviceInfo) GetSourceDevice() string {
	return filepath.Join("/dev", b.Name)
}

// filterGlobalMounts filters mount infos to only include CSI global mounts,
// excluding per-pod bind mounts. This handles storage providers like CephFS
// that create both a global mount and a per-pod bind mount.
func filterGlobalMounts(infos []FindmntInfo) []FindmntInfo {
	var globalMounts []FindmntInfo
	for _, info := range infos {
		if strings.Contains(info.Target, "/globalmount") {
			globalMounts = append(globalMounts, info)
		}
	}
	return globalMounts
}
