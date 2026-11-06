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
	volumes := buildBlockDeviceVolumesFromStoragePoolInfo(pools)
	mounts := buildBlockDeviceMountsFromStoragePoolInfo(pools)
	if len(volumes) != 1 || len(mounts) != 1 {
		t.Fatalf("expected one block device volume and mount, got %d and %d", len(volumes), len(mounts))
	}
	want := filepath.Join("/dev", blockDeviceAlias(longName))
	if volumes[0].HostPath.Path != want || mounts[0].MountPath != want {
		t.Fatalf("pool, host and container device paths must all equal %s", want)
	}
}
