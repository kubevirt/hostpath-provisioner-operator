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
package v1beta1

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

const (
	versionString   = "1.0.1"
	csiVolume       = "csi-data-dir"
	legacyVolume    = "pv-volume"
	longPathMax     = "/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/1234"
	longPathOverMax = "/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/123456789/12345"
)

var (
	bothLegacyAndVolumeCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			PathConfig: &PathConfig{
				Path: "test",
			},
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: "test",
				},
			},
		},
	}

	invalidPathConfigCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			PathConfig: &PathConfig{},
		},
	}

	multiSourceVolumeCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: "test",
				},
				{
					Name: "test2",
					Path: "test2",
				},
			},
		},
	}

	multiSourceVolumeDuplicatePathCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: "test",
				},
				{
					Name: "test2",
					Path: "test2",
				},
				{
					Name: "test3",
					Path: "test",
				},
			},
		},
	}

	multiSourceVolumeDuplicateNameCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: "test",
				},
				{
					Name: "test2",
					Path: "test2",
				},
				{
					Name: "test",
					Path: "test3",
				},
			},
		},
	}

	blankNameCr1 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Path: "test",
				},
			},
		},
	}
	blankNameCr2 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "",
					Path: "test",
				},
			},
		},
	}
	blankPathCr1 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
				},
			},
		},
	}
	blankPathCr2 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: "",
				},
			},
		},
	}
	storageClassCr = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name:        "test",
					Path:        "test",
					PVCTemplate: &corev1.PersistentVolumeClaimSpec{},
				},
			},
		},
	}
	longNameCr = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "1234567890123456789012345678901234567890123456789",
					Path: "test",
				},
				{
					Name: "l12345678901234567890123456789012345678901234567890",
					Path: "test2",
				},
			},
		},
	}
	longPathCr = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Name: "test",
					Path: longPathMax,
				},
				{
					Name: "test2",
					Path: longPathOverMax,
				},
			},
		},
	}
)

var _ = Describe("validating webhook", func() {
	Context("admission", func() {
		It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			Expect(hppCr.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		It("Both legacy or volume sources cannot to be set.", func() {
			Expect(bothLegacyAndVolumeCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankNameCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			Expect(blankNameCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			Expect(blankPathCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		It("If pathConfig exists, path must be set", func() {
			Expect(invalidPathConfigCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("pathconfig path must be set")))
		})
		It("Should not allow duplicate paths", func() {
			Expect(multiSourceVolumeDuplicatePathCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		It("Should not allow duplicate names", func() {
			Expect(multiSourceVolumeDuplicateNameCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		It("Should not allow storagepool.name length > 50", func() {
			Expect(longNameCr.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		It("Should not allow storagepool.path length > 255", func() {
			Expect(longPathCr.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})

	Context("update", func() {
		It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			Expect(hppCr.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		It("Both legacy or volume sources cannot to be set.", func() {
			Expect(bothLegacyAndVolumeCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankNameCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			Expect(blankNameCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			Expect(blankPathCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		It("Should not allow duplicate paths", func() {
			Expect(multiSourceVolumeDuplicatePathCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		It("Should not allow duplicate names", func() {
			Expect(multiSourceVolumeDuplicateNameCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		It("Should not allow storagepool.name length > 50", func() {
			Expect(longNameCr.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		It("Should not allow storagepool.path length > 255", func() {
			Expect(longPathCr.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})
})
