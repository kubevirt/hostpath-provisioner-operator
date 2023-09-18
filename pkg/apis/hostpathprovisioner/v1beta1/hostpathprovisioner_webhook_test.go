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

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
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

var _ = ginkgo.Describe("validating webhook", func() {
	ginkgo.Context("admission", func() {
		ginkgo.It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			_, err := hppCr.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		ginkgo.It("Both legacy or volume sources cannot to be set.", func() {
			_, err := bothLegacyAndVolumeCR.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		ginkgo.It("Cannot have blank kind in volume source", func() {
			_, err := blankNameCr1.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			_, err = blankNameCr2.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		ginkgo.It("Cannot have blank path in volume source", func() {
			_, err := blankPathCr1.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			_, err = blankPathCr2.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		ginkgo.It("If pathConfig exists, path must be set", func() {
			_, err := invalidPathConfigCR.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathconfig path must be set")))
		})
		ginkgo.It("Should not allow duplicate paths", func() {
			_, err := multiSourceVolumeDuplicatePathCR.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		ginkgo.It("Should not allow duplicate names", func() {
			_, err := multiSourceVolumeDuplicateNameCR.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		ginkgo.It("Should not allow storagepool.name length > 50", func() {
			_, err := longNameCr.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		ginkgo.It("Should not allow storagepool.path length > 255", func() {
			_, err := longPathCr.ValidateCreate()
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})

	ginkgo.Context("update", func() {
		ginkgo.It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			_, err := hppCr.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		ginkgo.It("Both legacy or volume sources cannot to be set.", func() {
			_, err := bothLegacyAndVolumeCR.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		ginkgo.It("Cannot have blank kind in volume source", func() {
			_, err := blankNameCr1.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			_, err = blankNameCr2.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		ginkgo.It("Cannot have blank path in volume source", func() {
			_, err := blankPathCr1.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			_, err = blankPathCr2.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		ginkgo.It("Should not allow duplicate paths", func() {
			_, err := multiSourceVolumeDuplicatePathCR.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		ginkgo.It("Should not allow duplicate names", func() {
			_, err := multiSourceVolumeDuplicateNameCR.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		ginkgo.It("Should not allow storagepool.name length > 50", func() {
			_, err := longNameCr.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		ginkgo.It("Should not allow storagepool.path length > 255", func() {
			_, err := longPathCr.ValidateUpdate(&HostPathProvisioner{})
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})
})
