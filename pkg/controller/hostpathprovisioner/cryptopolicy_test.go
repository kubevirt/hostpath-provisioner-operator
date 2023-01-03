/*
Copyright 2022 The hostpath provisioner operator Authors.

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
	"os"
	"strings"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	secv1 "github.com/openshift/api/security/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller reconcile loop", func() {
	Context("crypto policy", func() {
		BeforeEach(func() {
			watchNamespaceFunc = func() (string, error) {
				return testNamespace, nil
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		It("Should respect cluster-wide crypto config", func() {
			apiServer := &ocpconfigv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: ocpconfigv1.APIServerSpec{
					TLSSecurityProfile: &ocpconfigv1.TLSSecurityProfile{
						Type:   ocpconfigv1.TLSProfileModernType,
						Modern: &ocpconfigv1.ModernTLSProfile{},
					},
				},
			}
			nn := client.ObjectKeyFromObject(apiServer)
			// Register operator types with the runtime scheme.
			s := scheme.Scheme
			s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisioner{})
			s.AddKnownTypes(hppv1.SchemeGroupVersion, &hppv1.HostPathProvisionerList{})
			promv1.AddToScheme(s)
			secv1.Install(s)
			ocpconfigv1.Install(s)

			// Create a fake client to mock API calls.
			cl := erroringFakeCtrlRuntimeClient{
				Client: fake.NewFakeClientWithScheme(s, apiServer),
				errMsg: "",
			}

			// Mimic the watch handle func being called
			handleAPIServerFunc(apiServer)
			// Verify that crypto config is respected
			Expect(os.Getenv("TLS_MIN_VERSION")).To(Equal("VersionTLS13"))
			Expect(os.Getenv("TLS_CIPHERS")).To(Equal(strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileModernType].Ciphers, ",")))
			// Now modify the crypto config to something else
			err := cl.Get(context.TODO(), nn, apiServer)
			Expect(err).NotTo(HaveOccurred())
			apiServer.Spec.TLSSecurityProfile = &ocpconfigv1.TLSSecurityProfile{
				Type: ocpconfigv1.TLSProfileOldType,
				Old:  &ocpconfigv1.OldTLSProfile{},
			}
			err = cl.Update(context.TODO(), apiServer)
			Expect(err).NotTo(HaveOccurred())
			// Mimic the watch handle func being called
			handleAPIServerFunc(apiServer)
			// Verify changes are respected
			Expect(os.Getenv("TLS_MIN_VERSION")).To(Equal("VersionTLS10"))
			Expect(os.Getenv("TLS_CIPHERS")).To(Equal(strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileOldType].Ciphers, ",")))
		})
	})
})
