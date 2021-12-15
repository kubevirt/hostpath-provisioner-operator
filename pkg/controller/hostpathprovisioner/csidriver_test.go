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
package hostpathprovisioner

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller reconcile loop", func() {
	Context("csidriver", func() {
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		table.DescribeTable("Should not reconcile over immutable csidriver fields", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			csiDriverNN := types.NamespacedName{
				Name: "kubevirt.io.hostpath-provisioner",
			}
			_, r, cl := createDeployedCr(cr)
			// Now modify the CSIDriver to something not desired.
			csiDriver := &storagev1.CSIDriver{}
			err := cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			changedFSGroupPolicy := storagev1.FileFSGroupPolicy
			csiDriver.Spec.FSGroupPolicy = &changedFSGroupPolicy
			err = cl.Update(context.TODO(), csiDriver)
			Expect(err).NotTo(HaveOccurred())
			// Verify FSGroupPolicy is "File"
			csiDriver = &storagev1.CSIDriver{}
			err = cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			Expect(*csiDriver.Spec.FSGroupPolicy).To(Equal(storagev1.FileFSGroupPolicy))
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
			// Verify FSGroupPolicy stays the same, as it is an immutable field and we don't have to reconcile it
			csiDriver = &storagev1.CSIDriver{}
			err = cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			Expect(*csiDriver.Spec.FSGroupPolicy).To(Equal(storagev1.FileFSGroupPolicy))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		table.DescribeTable("Should fix a changed CSIDriver", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			csiDriverNN := types.NamespacedName{
				Name: "kubevirt.io.hostpath-provisioner",
			}
			_, r, cl := createDeployedCr(cr)
			// Now modify one of the mutable CSIDriver fields to something not desired.
			csiDriver := &storagev1.CSIDriver{}
			err := cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			requiresRepublish := true
			csiDriver.Spec.RequiresRepublish = &requiresRepublish
			err = cl.Update(context.TODO(), csiDriver)
			Expect(err).NotTo(HaveOccurred())
			// Verify requiresRepublish is true
			csiDriver = &storagev1.CSIDriver{}
			err = cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			Expect(*csiDriver.Spec.RequiresRepublish).To(BeTrue())
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
			// Verify requiresRepublish is false (default)
			csiDriver = &storagev1.CSIDriver{}
			err = cl.Get(context.TODO(), csiDriverNN, csiDriver)
			Expect(err).NotTo(HaveOccurred())
			Expect(*csiDriver.Spec.RequiresRepublish).To(BeFalse())
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)
	})
})
