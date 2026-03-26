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

		ginkgo.It("Metrics server should respect cluster-wide crypto config but enforce TLS 1.3 minimum", func() {
			// Clear environment variables first
			os.Unsetenv("TLS_CIPHERS_OVERRIDE")
			os.Unsetenv("TLS_MIN_VERSION_OVERRIDE")
			os.Unsetenv("TLS_CIPHERS")
			os.Unsetenv("TLS_MIN_VERSION")

			// Test 1: Default configuration (no env vars) should use TLS 1.3
			metricsOpts := cryptopolicy.GetMetricsServerOptions()
			gomega.Expect(metricsOpts.SecureServing).To(gomega.BeTrue())
			gomega.Expect(metricsOpts.BindAddress).To(gomega.Equal(":8443"))
			gomega.Expect(metricsOpts.TLSOpts).NotTo(gomega.BeEmpty())

			// Apply TLS options and verify default is TLS 1.3
			testConfig := &tls.Config{}
			for _, opt := range metricsOpts.TLSOpts {
				opt(testConfig)
			}
			// Verify GetConfigForClient sets TLS 1.3 when no env vars are set
			clientConfig, err := testConfig.GetConfigForClient(nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(clientConfig.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS13)))

			// Test 2: Simulate APIServer watch setting Modern profile (TLS 1.3)
			os.Setenv("TLS_MIN_VERSION", "VersionTLS13")
			os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileModernType].Ciphers, ","))

			testConfig2 := &tls.Config{}
			for _, opt := range metricsOpts.TLSOpts {
				opt(testConfig2)
			}
			clientConfig2, err := testConfig2.GetConfigForClient(nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(clientConfig2.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS13)))

			// Test 3: CRITICAL - Simulate APIServer watch setting Intermediate profile (TLS 1.2)
			// Should still enforce TLS 1.3 minimum for metrics endpoint
			os.Setenv("TLS_MIN_VERSION", "VersionTLS12")
			os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType].Ciphers, ","))

			testConfig3 := &tls.Config{}
			for _, opt := range metricsOpts.TLSOpts {
				opt(testConfig3)
			}
			clientConfig3, err := testConfig3.GetConfigForClient(nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Metrics endpoint enforces TLS 1.3 minimum even when cluster uses TLS 1.2
			gomega.Expect(clientConfig3.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS13)))

			// Test 4: CRITICAL - Simulate Old profile (TLS 1.0) should be upgraded to TLS 1.3
			os.Setenv("TLS_MIN_VERSION", "VersionTLS10")
			os.Setenv("TLS_CIPHERS", strings.Join(ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileOldType].Ciphers, ","))

			testConfig4 := &tls.Config{}
			for _, opt := range metricsOpts.TLSOpts {
				opt(testConfig4)
			}
			clientConfig4, err := testConfig4.GetConfigForClient(nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Metrics endpoint enforces TLS 1.3 minimum even when cluster uses TLS 1.0
			gomega.Expect(clientConfig4.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS13)))

			// Test 5: Override with lower version should still be upgraded to TLS 1.3
			os.Setenv("TLS_MIN_VERSION_OVERRIDE", "VersionTLS11")

			metricsOptsOverride := cryptopolicy.GetMetricsServerOptions()
			testConfig5 := &tls.Config{}
			for _, opt := range metricsOptsOverride.TLSOpts {
				opt(testConfig5)
			}
			clientConfig5, err := testConfig5.GetConfigForClient(nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Even overrides are subject to TLS 1.3 minimum
			gomega.Expect(clientConfig5.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS13)))

			// Clean up
			os.Unsetenv("TLS_CIPHERS_OVERRIDE")
			os.Unsetenv("TLS_MIN_VERSION_OVERRIDE")
			os.Unsetenv("TLS_CIPHERS")
			os.Unsetenv("TLS_MIN_VERSION")
		})
	})
})
