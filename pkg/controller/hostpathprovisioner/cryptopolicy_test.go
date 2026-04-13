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
	"crypto/tls"
	"os"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	secv1 "github.com/openshift/api/security/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hppv1 "kubevirt.io/hostpath-provisioner-operator/pkg/apis/hostpathprovisioner/v1beta1"
	"kubevirt.io/hostpath-provisioner-operator/pkg/util/cryptopolicy"
	"kubevirt.io/hostpath-provisioner-operator/version"
)

var _ = ginkgo.Describe("Controller reconcile loop", func() {
	ginkgo.Context("crypto policy", func() {
		ginkgo.BeforeEach(func() {
			watchNamespaceFunc = func() string {
				return testNamespace
			}
			version.VersionStringFunc = func() (string, error) {
				return versionString, nil
			}
		})

		ginkgo.It("Should respect cluster-wide crypto config", func() {
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
				Client: fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(apiServer).Build(),
				errMsg: "",
			}

			// Mimic the watch handle func being called
			handleAPIServerFunc(context.TODO(), apiServer)
			// Verify that crypto config is respected
			gomega.Expect(os.Getenv("TLS_MIN_VERSION")).To(gomega.Equal("VersionTLS13"))
			gomega.Expect(os.Getenv("TLS_CIPHERS")).To(gomega.Equal(strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileModernType].Ciphers, ",")))
			// Now modify the crypto config to something else
			err := cl.Get(context.TODO(), nn, apiServer)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			apiServer.Spec.TLSSecurityProfile = &ocpconfigv1.TLSSecurityProfile{
				Type: ocpconfigv1.TLSProfileOldType,
				Old:  &ocpconfigv1.OldTLSProfile{},
			}
			err = cl.Update(context.TODO(), apiServer)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Mimic the watch handle func being called
			handleAPIServerFunc(context.TODO(), apiServer)
			// Verify changes are respected
			gomega.Expect(os.Getenv("TLS_MIN_VERSION")).To(gomega.Equal("VersionTLS10"))
			gomega.Expect(os.Getenv("TLS_CIPHERS")).To(gomega.Equal(strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileOldType].Ciphers, ",")))
		})

		ginkgo.Context("metrics server TLS configuration", func() {
			ginkgo.BeforeEach(func() {
				// Clear environment variables before each test
				os.Unsetenv("TLS_CIPHERS_OVERRIDE")
				os.Unsetenv("TLS_MIN_VERSION_OVERRIDE")
				os.Unsetenv("TLS_CIPHERS")
				os.Unsetenv("TLS_MIN_VERSION")
			})

			ginkgo.AfterEach(func() {
				// Clean up environment variables after each test
				os.Unsetenv("TLS_CIPHERS_OVERRIDE")
				os.Unsetenv("TLS_MIN_VERSION_OVERRIDE")
				os.Unsetenv("TLS_CIPHERS")
				os.Unsetenv("TLS_MIN_VERSION")
			})

			ginkgo.It("should be configured for secure serving", func() {
				metricsOpts := cryptopolicy.GetMetricsServerOptions()
				gomega.Expect(metricsOpts.SecureServing).To(gomega.BeTrue())
				gomega.Expect(metricsOpts.BindAddress).To(gomega.Equal(":8443"))
				gomega.Expect(metricsOpts.TLSOpts).NotTo(gomega.BeEmpty())
			})

			ginkgo.DescribeTable("should respect TLS configuration from cluster or override",
				func(setupEnv func(), expectedMinVersion uint16, description string) {
					// Setup environment for this test case
					if setupEnv != nil {
						setupEnv()
					}

					// Get metrics options and apply TLS config
					metricsOpts := cryptopolicy.GetMetricsServerOptions()
					testConfig := &tls.Config{}
					for _, opt := range metricsOpts.TLSOpts {
						opt(testConfig)
					}

					// Get config for client and verify expected TLS version
					clientConfig, err := testConfig.GetConfigForClient(nil)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(clientConfig.MinVersion).To(gomega.Equal(expectedMinVersion), description)
				},
				ginkgo.Entry("default configuration with no env vars",
					nil,
					uint16(0),
					"should have no MinVersion when no config is set"),
				ginkgo.Entry("Modern profile (TLS 1.3) from cluster",
					func() {
						os.Setenv("TLS_MIN_VERSION", "VersionTLS13")
						os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileModernType].Ciphers, ","))
					},
					uint16(tls.VersionTLS13),
					"should use TLS 1.3 from cluster"),
				ginkgo.Entry("Intermediate profile (TLS 1.2) from cluster",
					func() {
						os.Setenv("TLS_MIN_VERSION", "VersionTLS12")
						os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType].Ciphers, ","))
					},
					uint16(tls.VersionTLS12),
					"should respect TLS 1.2 from cluster"),
				ginkgo.Entry("Old profile (TLS 1.0) from cluster",
					func() {
						os.Setenv("TLS_MIN_VERSION", "VersionTLS10")
						os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileOldType].Ciphers, ","))
					},
					uint16(tls.VersionTLS10),
					"should respect TLS 1.0 from cluster"),
				ginkgo.Entry("Override to TLS 1.3",
					func() {
						os.Setenv("TLS_MIN_VERSION_OVERRIDE", "VersionTLS13")
					},
					uint16(tls.VersionTLS13),
					"should use TLS 1.3 from override"),
				ginkgo.Entry("Override takes precedence over cluster",
					func() {
						os.Setenv("TLS_MIN_VERSION", "VersionTLS10")
						os.Setenv("TLS_MIN_VERSION_OVERRIDE", "VersionTLS13")
					},
					uint16(tls.VersionTLS13),
					"should use override (TLS 1.3) instead of cluster (TLS 1.0)"),
			)
		})
	})
})
