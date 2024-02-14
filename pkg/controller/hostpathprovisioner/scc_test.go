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
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("scc", func() {
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.DescribeTable("Should fix a changed SecurityContextConstraints", func(cr *hppv1.HostPathProvisioner, names ...string) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			for _, name := range names {
				sccNN := types.NamespacedName{
					Name: name,
				}
				_, r, cl := createDeployedCr(cr)
				// Now modify the SCC to something not desired.
				scc := &secv1.SecurityContextConstraints{}
				err := cl.Get(context.TODO(), sccNN, scc)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				scc.AllowPrivilegedContainer = true
				err = cl.Update(context.TODO(), scc)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Verify allowPrivileged is true
				scc = &secv1.SecurityContextConstraints{}
				err = cl.Get(context.TODO(), sccNN, scc)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(scc.AllowPrivilegedContainer).To(gomega.BeTrue())
				// Run the reconcile loop
				res, err := r.Reconcile(context.TODO(), req)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(res.Requeue).To(gomega.BeFalse())
				// Verify allowPrivileged is false
				scc = &secv1.SecurityContextConstraints{}
				err = cl.Get(context.TODO(), sccNN, scc)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(scc.AllowPrivilegedContainer).To(gomega.Equal(MultiPurposeHostPathProvisionerName != name))
				if name == MultiPurposeHostPathProvisionerName {
					gomega.Expect(scc.Volumes).To(gomega.ContainElements(secv1.FSTypeHostPath, secv1.FSTypeSecret, secv1.FSProjected))
				} else {
					gomega.Expect(scc.Volumes).To(gomega.ContainElements(secv1.FSTypeAll))
				}
			}
		},
			ginkgo.Entry("legacyCr", createLegacyCr(), MultiPurposeHostPathProvisionerName, fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
			ginkgo.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr(), fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
			ginkgo.Entry("storagePoolCr", createStoragePoolWithTemplateCr(), fmt.Sprintf("%s-csi", MultiPurposeHostPathProvisionerName)),
		)
	})
})
