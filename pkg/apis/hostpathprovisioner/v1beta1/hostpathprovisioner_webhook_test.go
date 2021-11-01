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
	v1 "k8s.io/api/core/v1"
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
			VolumeSources: []VolumeSource{
				{
					Kind: "test",
					Path: "test",
				},
			},
		},
	}

	multiSourceVolumeCR = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Kind: "test",
					Path: "test",
				},
				{
					Kind: "test2",
					Path: "test2",
				},
			},
		},
	}

	blankKindCr1 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Path: "test",
				},
			},
		},
	}
	blankKindCr2 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Kind: "",
					Path: "test",
				},
			},
		},
	}
	blankPathCr1 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Kind: "test",
				},
			},
		},
	}
	blankPathCr2 = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Kind: "test",
					Path: "",
				},
			},
		},
	}
	bothPVCandStorageClassCr = HostPathProvisioner{
		Spec: HostPathProvisionerSpec{
			VolumeSources: []VolumeSource{
				{
					Kind:               "test",
					Path:               "test",
					SourceStorageClass: &SourceStorageClass{},
					PVC:                &v1.PersistentVolumeClaimSpec{},
				},
			},
		},
	}
)

var _ = Describe("validating webhook", func() {
	Context("admission", func() {
		It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			Expect(hppCr.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("either pathConfig or volumeSources must be set")))
		})
		It("Both legacy or volume sources cannot to be set.", func() {
			Expect(bothLegacyAndVolumeCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("pathConfig and volumeSources cannot be both set")))
		})
		It("Cannot have more than one volume source", func() {
			Expect(multiSourceVolumeCR.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("currently only 1 volume source is supported")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankKindCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("volumesource.kind cannot be blank")))
			Expect(blankKindCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("volumesource.kind cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("volumesource.path cannot be blank")))
			Expect(blankPathCr2.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("volumesource.path cannot be blank")))
		})
		It("Cannot have PVC and sourceStorageClass set", func() {
			Expect(bothPVCandStorageClassCr.ValidateCreate()).To(BeEquivalentTo(fmt.Errorf("both volumesource.PVC and volumesource.sourceStorageClass cannot be set")))
		})
	})

	Context("update", func() {
		It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			Expect(hppCr.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("either pathConfig or volumeSources must be set")))
		})
		It("Both legacy or volume sources cannot to be set.", func() {
			Expect(bothLegacyAndVolumeCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("pathConfig and volumeSources cannot be both set")))
		})
		It("Cannot have more than one volume source", func() {
			Expect(multiSourceVolumeCR.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("currently only 1 volume source is supported")))
		})
		It("Cannot have blank kind in volume source", func() {
			Expect(blankKindCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("volumesource.kind cannot be blank")))
			Expect(blankKindCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("volumesource.kind cannot be blank")))
		})
		It("Cannot have blank path in volume source", func() {
			Expect(blankPathCr1.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("volumesource.path cannot be blank")))
			Expect(blankPathCr2.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("volumesource.path cannot be blank")))
		})
		It("Cannot have PVC and sourceStorageClass set", func() {
			Expect(bothPVCandStorageClassCr.ValidateUpdate(&HostPathProvisioner{})).To(BeEquivalentTo(fmt.Errorf("both volumesource.PVC and volumesource.sourceStorageClass cannot be set")))
		})
	})
})
