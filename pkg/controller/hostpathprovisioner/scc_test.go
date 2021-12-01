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

	"k8s.io/apimachinery/pkg/types"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
)

var _ = Describe("Controller reconcile loop", func() {
	Context("scc", func() {
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		table.DescribeTable("Should fix a changed SecurityContextConstraints", func(cr *hppv1.HostPathProvisioner) {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-name",
					Namespace: testNamespace,
				},
			}
			sccNN := types.NamespacedName{
				Name: MultiPurposeHostPathProvisionerName,
			}
			cr, r, cl := createDeployedCr(cr)
			// Now modify the SCC to something not desired.
			scc := &secv1.SecurityContextConstraints{}
			err := cl.Get(context.TODO(), sccNN, scc)
			Expect(err).NotTo(HaveOccurred())
			scc.AllowPrivilegedContainer = true
			err = cl.Update(context.TODO(), scc)
			Expect(err).NotTo(HaveOccurred())
			// Verify allowPrivileged is true
			scc = &secv1.SecurityContextConstraints{}
			err = cl.Get(context.TODO(), sccNN, scc)
			Expect(err).NotTo(HaveOccurred())
			Expect(scc.AllowPrivilegedContainer).To(BeTrue())
			// Run the reconcile loop
			res, err := r.Reconcile(context.TODO(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Requeue).To(BeFalse())
			// Verify allowPrivileged is false
			scc = &secv1.SecurityContextConstraints{}
			err = cl.Get(context.TODO(), sccNN, scc)
			Expect(err).NotTo(HaveOccurred())
			Expect(scc.AllowPrivilegedContainer).To(BeFalse())
			Expect(scc.Volumes).To(ContainElements(secv1.FSTypeHostPath, secv1.FSTypeSecret, secv1.FSProjected))
		},
			table.Entry("legacyCr", createLegacyCr()),
			table.Entry("legacyStoragePoolCr", createLegacyStoragePoolCr()),
			table.Entry("storagePoolCr", createStoragePoolWithTemplateCr()),
		)

	})
})
