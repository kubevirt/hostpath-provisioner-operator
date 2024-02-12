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

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("rbac", func() {
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.DescribeTable("Should fix a changed ClusterRole", func(cr *hppv1.HostPathProvisioner) {
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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(len(crole.Rules[1].Verbs)).To(gomega.Equal(4))
			// Add delete to persistentvolumeclaims rule
			crole.Rules[1].Verbs = append(crole.Rules[1].Verbs, "delete")
			err = cl.Update(context.TODO(), crole)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			crole = &rbacv1.ClusterRole{}
			err = cl.Get(context.TODO(), croleNN, crole)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Verify the extra ability is there.
			gomega.Expect(len(crole.Rules[1].Verbs)).To(gomega.Equal(5))
			gomega.Expect(crole.Rules[1].Verbs[4]).To(gomega.Equal("delete"))
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())
			// Verify its gone now
			err = cl.Get(context.TODO(), croleNN, crole)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(len(crole.Rules[1].Verbs)).To(gomega.Equal(4))
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		ginkgo.DescribeTable("Should modify ClusterRole if snapshot enabled", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			cr, r, cl := createDeployedCr(cr)

			cr = &hppv1.HostPathProvisioner{}
			err := r.client.Get(context.TODO(), req.NamespacedName, cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Update the CR to enable the snapshotting feature gate.
			cr.Spec.FeatureGates = append(cr.Spec.FeatureGates, snapshotFeatureGate)
			err = cl.Update(context.TODO(), cr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())

			verifyCreateCSIClusterRole(cl, true)
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

		ginkgo.DescribeTable("Should fix a changed ClusterRoleBinding", func(cr *hppv1.HostPathProvisioner) {
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
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			crb.Subjects[0].Name = "invalid"
			err = cl.Update(context.TODO(), crb)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Verify the name is wrong
			crb = &rbacv1.ClusterRoleBinding{}
			err = cl.Get(context.TODO(), crbNN, crb)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(crb.Subjects[0].Name).To(gomega.Equal("invalid"))
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res.Requeue).To(gomega.BeFalse())
			// Verify the name is correct again.
			crb = &rbacv1.ClusterRoleBinding{}
			err = cl.Get(context.TODO(), crbNN, crb)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(crb.Subjects[0].Name).To(gomega.Equal(ProvisionerServiceAccountNameCsi))
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)
	})
})
