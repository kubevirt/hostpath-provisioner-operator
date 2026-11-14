package hostpathprovisioner

import (
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func TestStoragePoolDevicePaths(t *testing.T) {
	block := corev1.PersistentVolumeBlock
	filesystem := corev1.PersistentVolumeFilesystem
	longName := strings.Repeat("pool", 30)
	cr := &hppv1.HostPathProvisioner{
		Spec: hppv1.HostPathProvisionerSpec{
			StoragePools: []hppv1.StoragePool{
				{Name: longName, Path: "/block", PVCTemplate: &corev1.PersistentVolumeClaimSpec{VolumeMode: &block}},
				{Name: "filesystem", Path: "/filesystem", PVCTemplate: &corev1.PersistentVolumeClaimSpec{VolumeMode: &filesystem}},
				{Name: "directory", Path: "/directory"},
			},
		},
	}
	pools := getStoragePoolPaths(cr)
	deviceVolumes := buildBlockDeviceVolumesFromStoragePoolInfo(pools)
	devicePaths := buildBlockDevicePathsFromStoragePoolInfo(pools)
	if len(deviceVolumes) != 1 || len(devicePaths) != 1 {
		t.Fatalf("expected one block device volume and device path, got %d and %d", len(deviceVolumes), len(devicePaths))
	}

	poolVolumes := buildVolumesFromStoragePoolInfo(pools)
	poolMounts := buildVolumeMountsFromStoragePoolInfo(pools)
	if len(poolVolumes) != len(pools) || len(poolMounts) != len(pools) {
		t.Fatalf("expected %d pool directory volumes and mounts, got %d and %d", len(pools), len(poolVolumes), len(poolMounts))
	}
	for i, pool := range pools {
		wantVolumeName := getMountNameFromStoragePool(pool.Name)
		if poolVolumes[i].Name != wantVolumeName || poolVolumes[i].HostPath == nil || poolVolumes[i].HostPath.Path != pool.Path {
			t.Errorf("pool volume %d should map %q to %q, got %#v", i, pool.Path, wantVolumeName, poolVolumes[i])
		}
		wantMountPath := filepath.Join("/", wantVolumeName)
		if poolMounts[i].Name != wantVolumeName || poolMounts[i].MountPath != wantMountPath {
			t.Errorf("pool mount %d should map volume %q to %q, got %#v", i, wantVolumeName, wantMountPath, poolMounts[i])
		}
	}

	alias := blockDeviceAlias(longName)
	if len(alias) != maxNameLength {
		t.Fatalf("expected long pool alias to be capped at %d characters, got %d: %q", maxNameLength, len(alias), alias)
	}
	wantAlias := longName[:maxNameLength-10-len("device")] + "-" + hash(longName) + "-device"
	if alias != wantAlias {
		t.Fatalf("expected long pool alias %q to include the generated hash, got %q", wantAlias, alias)
	}

	want := filepath.Join("/dev", alias)
	if deviceVolumes[0].HostPath.Path != want || devicePaths[0].DevicePath != want {
		t.Fatalf("pool, host and container device paths must all equal %s", want)
	}
	if devicePaths[0].Name != deviceVolumes[0].Name {
		t.Fatalf("device path volume name %q does not match host volume name %q", devicePaths[0].Name, deviceVolumes[0].Name)
	}
}
