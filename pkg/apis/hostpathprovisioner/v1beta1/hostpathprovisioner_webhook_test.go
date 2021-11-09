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
)

const (
	versionString = "1.0.1"
	csiVolume     = "csi-data-dir"
	legacyVolume  = "pv-volume"
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

	blankKindCr1 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			StoragePools: []StoragePool{
				{
					Path: "test",
				},
			},
		},
	}
	blankKindCr2 = HostPathProvisioner{
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
					Name:         "test",
					Path:         "test",
					StorageClass: &SourceStorageClass{},
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
		It("Cannot have more than one volume source", func() {
			Expect(multiSourceVolumeCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("currently only 1 storage pool is supported")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankKindCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.kind cannot be blank")))
			Expect(blankKindCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.kind cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			Expect(blankPathCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
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
		It("Cannot have more than one volume source", func() {
			Expect(multiSourceVolumeCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("currently only 1 storage pool is supported")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankKindCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.kind cannot be blank")))
			Expect(blankKindCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.kind cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			Expect(blankPathCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
	})
})
