package hostpathprovisioner

import (
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
)

func TestStoragePoolDirectoryPaths(t *testing.T) {
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
}
