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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("service account", func() {
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.DescribeTable("Should fix a changed service account", func(cr *hppv1.HostPathProvisioner, saNames ...string) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			_, r, cl := createDeployedCr(cr)
			for _, saName := range saNames {
				// Now modify the service account to something not desired.
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      saName,
						Namespace: testNamespace,
					},
				}
				err := cl.Get(context.TODO(), client.ObjectKeyFromObject(sa), sa)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(sa.ObjectMeta.Labels["k8s-app"]).To(gomega.Equal(MultiPurposeHostPathProvisionerName))
				sa.ObjectMeta.Labels["k8s-app"] = "invalid"
				err = cl.Update(context.TODO(), sa)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				sa = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      saName,
						Namespace: testNamespace,
					},
				}
				err = cl.Get(context.TODO(), client.ObjectKeyFromObject(sa), sa)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(sa.ObjectMeta.Labels["k8s-app"]).To(gomega.Equal("invalid"))
				// Run the reconcile loop
				res, err := r.Reconcile(context.TODO(), req)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(res.Requeue).To(gomega.BeFalse())
				// Verify the label has been changed back.
				sa = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      saName,
						Namespace: testNamespace,
					},
				}
				err = cl.Get(context.TODO(), client.ObjectKeyFromObject(sa), sa)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(sa.ObjectMeta.Labels["k8s-app"]).To(gomega.Equal(MultiPurposeHostPathProvisionerName))
			}
		},
			ginkgo.Entry("legacyCr", createLegacyCr()),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)
	})
})
