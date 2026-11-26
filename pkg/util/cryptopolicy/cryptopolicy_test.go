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
package cryptopolicy

import (
	"crypto/tls"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
)

func TestCryptoPolicy(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "CryptoPolicy Suite")
}

var _ = ginkgo.Describe("TLS crypto watch", func() {
	ginkgo.Context("groupsToCurves", func() {
		ginkgo.DescribeTable("should convert", func(group ocpconfigv1.TLSGroup, curve tls.CurveID) {
			gomega.Expect(groupsToCurves([]ocpconfigv1.TLSGroup{group})).To(gomega.Equal([]tls.CurveID{curve}))
		},
			ginkgo.EntryDescription("%s to %s"),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupX25519, tls.X25519),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupSecP256r1, tls.CurveP256),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupSecP384r1, tls.CurveP384),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupSecP521r1, tls.CurveP521),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupX25519MLKEM768, tls.X25519MLKEM768),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupSecP256r1MLKEM768, tls.SecP256r1MLKEM768),
			ginkgo.Entry(nil, ocpconfigv1.TLSGroupSecP384r1MLKEM1024, tls.SecP384r1MLKEM1024),
		)

		ginkgo.It("should return no curves for a nil list", func() {
			gomega.Expect(groupsToCurves(nil)).To(gomega.BeEmpty())
		})

		ginkgo.It("should map every group of the built-in profiles", func() {
			for profileType, spec := range ocpconfigv1.TLSProfiles {
				gomega.Expect(groupsToCurves(spec.Groups)).To(gomega.HaveLen(len(spec.Groups)), "profile %s", profileType)
			}
		})
	})

	ginkgo.Context("getTLSProfileSpec", func() {
		expectSpec := func(tlsSecurityProfile *ocpconfigv1.TLSSecurityProfile, spec *ocpconfigv1.TLSProfileSpec) {
			tlsProfileSpec := getTLSProfileSpec(tlsSecurityProfile)
			gomega.Expect(tlsProfileSpec.Ciphers).To(gomega.Equal(spec.Ciphers))
			gomega.Expect(tlsProfileSpec.MinTLSVersion).To(gomega.Equal(spec.MinTLSVersion))
			gomega.Expect(tlsProfileSpec.Groups).To(gomega.Equal(spec.Groups))
		}

		ginkgo.DescribeTable("should fall back to Intermediate for an incomplete profile", func(tlsSecurityProfile *ocpconfigv1.TLSSecurityProfile) {
			expectSpec(tlsSecurityProfile, ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType])
		},
			ginkgo.Entry("nil profile", nil),
			ginkgo.Entry("empty profile", &ocpconfigv1.TLSSecurityProfile{}),
			ginkgo.Entry("Custom without a custom spec", &ocpconfigv1.TLSSecurityProfile{Type: ocpconfigv1.TLSProfileCustomType}),
		)

		ginkgo.DescribeTable("should return the built-in spec for a known type", func(tlsSecurityProfile *ocpconfigv1.TLSSecurityProfile) {
			expectSpec(tlsSecurityProfile, ocpconfigv1.TLSProfiles[tlsSecurityProfile.Type])
		},
			ginkgo.Entry("Old", &ocpconfigv1.TLSSecurityProfile{Type: ocpconfigv1.TLSProfileOldType, Old: &ocpconfigv1.OldTLSProfile{}}),
			ginkgo.Entry("Intermediate", &ocpconfigv1.TLSSecurityProfile{Type: ocpconfigv1.TLSProfileIntermediateType, Intermediate: &ocpconfigv1.IntermediateTLSProfile{}}),
			ginkgo.Entry("Modern", &ocpconfigv1.TLSSecurityProfile{Type: ocpconfigv1.TLSProfileModernType, Modern: &ocpconfigv1.ModernTLSProfile{}}),
		)

		ginkgo.It("should return the custom spec for a Custom profile", func() {
			custom := ocpconfigv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256"},
				MinTLSVersion: ocpconfigv1.VersionTLS11,
				Groups:        []ocpconfigv1.TLSGroup{ocpconfigv1.TLSGroupSecP521r1},
			}
			p := &ocpconfigv1.TLSSecurityProfile{
				Type:   ocpconfigv1.TLSProfileCustomType,
				Custom: &ocpconfigv1.CustomTLSProfile{TLSProfileSpec: custom},
			}
			expectSpec(p, &custom)
		})
	})
})
