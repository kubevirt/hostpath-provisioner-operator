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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller reconcile loop", func() {
	Context("rbac", func() {
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		table.DescribeTable("Should fix a changed ClusterRole", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			name := ProvisionerServiceAccountNameCsi
			croleNN := types.NamespacedName{
				Name: name,
			}
			cr, r, cl := createDeployedCr(cr)
			// Now modify the ClusterRole to something not desired.
			crole := &rbacv1.ClusterRole{}
			err := cl.Get(context.TODO(), croleNN, crole)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(crole.Rules[1].Verbs)).To(Equal(4))
			// Add delete to persistentvolumeclaims rule
			crole.Rules[1].Verbs = append(crole.Rules[1].Verbs, "delete")
			err = cl.Update(context.TODO(), crole)
			Expect(err).NotTo(HaveOccurred())
			crole = &rbacv1.ClusterRole{}
			err = cl.Get(context.TODO(), croleNN, crole)
			Expect(err).NotTo(HaveOccurred())
			// Verify the extra ability is there.
			Expect(len(crole.Rules[1].Verbs)).To(Equal(5))
			Expect(crole.Rules[1].Verbs[4]).To(Equal("delete"))
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
			// Verify its gone now
			err = cl.Get(context.TODO(), croleNN, crole)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(crole.Rules[1].Verbs)).To(Equal(4))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		table.DescribeTable("Should modify ClusterRole if snapshot enabled", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl := createDeployedCr(cr)

			cr = &hppv1.HostPathProvisioner{}
			err := r.client.Get(context.TODO(), req.NamespacedName, cr)
			Expect(err).NotTo(HaveOccurred())

			// Update the CR to enable the snapshotting feature gate.
			cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
			err = cl.Update(context.TODO(), cr)
			Expect(err).NotTo(HaveOccurred())
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())

			verifyCreateCSIClusterRole(cl, true)
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		table.DescribeTable("Should fix a changed ClusterRoleBinding", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			name := ProvisionerServiceAccountNameCsi
			crbNN := types.NamespacedName{
				Name: name,
			}
			cr, r, cl := createDeployedCr(cr)

			// Now modify the CRB to something not desired.
			crb := &rbacv1.ClusterRoleBinding{}
			err := cl.Get(context.TODO(), crbNN, crb)
			Expect(err).NotTo(HaveOccurred())
			crb.Subjects[0].Name = "invalid"
			err = cl.Update(context.TODO(), crb)
			Expect(err).NotTo(HaveOccurred())
			// Verify the name is wrong
			crb = &rbacv1.ClusterRoleBinding{}
			err = cl.Get(context.TODO(), crbNN, crb)
			Expect(err).NotTo(HaveOccurred())
			Expect(crb.Subjects[0].Name).To(Equal("invalid"))
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
			// Verify the name is correct again.
			crb = &rbacv1.ClusterRoleBinding{}
			err = cl.Get(context.TODO(), crbNN, crb)
			Expect(err).NotTo(HaveOccurred())
			Expect(crb.Subjects[0].Name).To(Equal(ProvisionerServiceAccountNameCsi))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)
	})
})
