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
	"context"
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &hppCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		ginkgo.It("Both legacy or volume sources cannot to be set.", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &bothLegacyAndVolumeCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		ginkgo.It("Cannot have blank kind in volume source", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &blankNameCr1)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			_, err = hppCrValidator.ValidateCreate(context.Background(), &blankNameCr2)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		ginkgo.It("Cannot have blank path in volume source", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &blankPathCr1)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			_, err = hppCrValidator.ValidateCreate(context.Background(), &blankPathCr2)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		ginkgo.It("If pathConfig exists, path must be set", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &invalidPathConfigCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathconfig path must be set")))
		})
		ginkgo.It("Should not allow duplicate paths", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &multiSourceVolumeDuplicatePathCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		ginkgo.It("Should not allow duplicate names", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &multiSourceVolumeDuplicateNameCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		ginkgo.It("Should not allow storagepool.name length > 50", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &longNameCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		ginkgo.It("Should not allow storagepool.path length > 255", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateCreate(context.Background(), &longPathCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})

	ginkgo.Context("update", func() {
		ginkgo.It("Either legacy or volume sources have to be set.", func() {
			hppCr := HostPathProvisioner{}
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &hppCr, &hppCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("either pathConfig or storage pools must be set")))
		})
		ginkgo.It("Both legacy or volume sources cannot to be set.", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &bothLegacyAndVolumeCR, &bothLegacyAndVolumeCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("pathConfig and storage pools cannot be both set")))
		})
		ginkgo.It("Cannot have blank kind in volume source", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &blankNameCr1, &blankNameCr1)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
			_, err = hppCrValidator.ValidateUpdate(context.Background(), &blankNameCr2, &blankNameCr2)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot be blank")))
		})
		ginkgo.It("Cannot have blank path in volume source", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &blankPathCr1, &blankPathCr1)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
			_, err = hppCrValidator.ValidateUpdate(context.Background(), &blankPathCr2, &blankPathCr2)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot be blank")))
		})
		ginkgo.It("Should not allow duplicate paths", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &multiSourceVolumeDuplicatePathCR, &multiSourceVolumeDuplicatePathCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].path is the same as spec.storagePools[0].path, cannot have duplicate paths")))
		})
		ginkgo.It("Should not allow duplicate names", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &multiSourceVolumeDuplicateNameCR, &multiSourceVolumeDuplicateNameCR)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("spec.storagePools[2].name is the same as spec.storagePools[0].name, cannot have duplicate names")))
		})
		ginkgo.It("Should not allow storagepool.name length > 50", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &longNameCr, &longNameCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.name cannot have a length greater than 50")))
		})
		ginkgo.It("Should not allow storagepool.path length > 255", func() {
			hppCrValidator := HostPathProvisionerValidator{}
			_, err := hppCrValidator.ValidateUpdate(context.Background(), &longPathCr, &longPathCr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("storagePool.path cannot have a length greater than 255")))
		})
	})

	ginkgo.Context("resources", func() {
		storagePoolCRWithResources := func(resources *ComponentResources) *HostPathProvisioner {
			cr := multiSourceVolumeCR.DeepCopy()
			cr.Spec.Resources = resources
			return cr
		}

		ginkgo.DescribeTable("Should allow valid resources", func(resources *ComponentResources) {
			hppCrValidator := HostPathProvisionerValidator{}
			cr := storagePoolCRWithResources(resources)
			_, err := hppCrValidator.ValidateCreate(context.Background(), cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = hppCrValidator.ValidateUpdate(context.Background(), cr, cr)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		},
			ginkgo.Entry("no resources", nil),
			ginkgo.Entry("empty resources", &ComponentResources{}),
			ginkgo.Entry("empty requirements", &ComponentResources{CSIProvisioner: &corev1.ResourceRequirements{}}),
			ginkgo.Entry("requests and limits", &ComponentResources{
				HostPathProvisioner: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("150Mi")},
					Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			}),
			ginkgo.Entry("limits only", &ComponentResources{
				Mounter: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("50Mi")},
				},
			}),
			ginkgo.Entry("requests only", &ComponentResources{
				LivenessProbe: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("50Mi")},
				},
			}),
		)

		ginkgo.DescribeTable("Should reject invalid resources", func(resources *ComponentResources, expectedErr string) {
			hppCrValidator := HostPathProvisionerValidator{}
			cr := storagePoolCRWithResources(resources)
			_, err := hppCrValidator.ValidateCreate(context.Background(), cr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("%s", expectedErr)))
			_, err = hppCrValidator.ValidateUpdate(context.Background(), cr, cr)
			gomega.Expect(err).To(gomega.BeEquivalentTo(fmt.Errorf("%s", expectedErr)))
		},
			ginkgo.Entry("request greater than limit", &ComponentResources{
				CSIProvisioner: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("200Mi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("100Mi")},
				},
			}, "spec.resources.csiProvisioner.requests.memory must be less than or equal to spec.resources.csiProvisioner.limits.memory"),
			ginkgo.Entry("negative request", &ComponentResources{
				NodeDriverRegistrar: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("-1m")},
				},
			}, "spec.resources.nodeDriverRegistrar.requests.cpu cannot be negative"),
			ginkgo.Entry("negative limit", &ComponentResources{
				CSISnapshotter: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("-1")},
				},
			}, "spec.resources.csiSnapshotter.limits.cpu cannot be negative"),
		)
	})
})
