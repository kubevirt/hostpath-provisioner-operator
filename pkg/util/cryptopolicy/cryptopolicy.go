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
	"os"
	"strings"

	ocpconfigv1 "github.com/openshift/api/config/v1"

	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func GetWebhookServerSpec() *webhook.Server {
	ciphersNames := strings.Split(os.Getenv("TLS_CIPHERS_OVERRIDE"), ",")
	ciphers := cipherSuitesIDs(ciphersNames)
	minTLSVersion := getTLSVersion(os.Getenv("TLS_MIN_VERSION_OVERRIDE"))

	tlsCfgMutateFunc := func(cfg *tls.Config) {
		if len(ciphers) != 0 {
			cfg.CipherSuites = ciphers
		}
		if minTLSVersion != nil {
			cfg.MinVersion = *minTLSVersion
		}
		// This callback executes on each client call returning a new config to be used
		cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			if os.Getenv("TLS_CIPHERS_OVERRIDE") == "" {
				// set CipherSuites if they were not set already
				ciphersNames := strings.Split(os.Getenv("TLS_CIPHERS"), ",")
				ciphers := cipherSuitesIDs(ciphersNames)
				if len(ciphers) != 0 {
					cfg.CipherSuites = ciphers
				}
			}
			if os.Getenv("TLS_MIN_VERSION_OVERRIDE") == "" {
				// set MinVersion if it was not set already
				minTLSVersion := getTLSVersion(os.Getenv("TLS_MIN_VERSION"))
				if minTLSVersion != nil {
					cfg.MinVersion = *minTLSVersion
				}
			}

			return cfg, nil
		}
	}

	return &webhook.Server{
		TLSOpts: []func(*tls.Config){
			tlsCfgMutateFunc,
		},
	}
}

func getTLSVersion(versionName string) *uint16 {
	var versions = map[string]uint16{
		"VersionTLS10": tls.VersionTLS10,
		"VersionTLS11": tls.VersionTLS11,
		"VersionTLS12": tls.VersionTLS12,
		"VersionTLS13": tls.VersionTLS13,
	}
	if version, ok := versions[versionName]; ok {
		return &version
	}

	return nil
}

func cipherSuitesIDs(names []string) []uint16 {
	// ref: https://www.iana.org/assignments/tls-parameters/tls-parameters.xml
	var idByName = map[string]uint16{
		// TLS 1.2
		"ECDHE-ECDSA-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-RSA-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-ECDSA-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-RSA-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-ECDSA-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-RSA-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-ECDSA-AES128-SHA256":     tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-RSA-AES128-SHA256":       tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"AES128-GCM-SHA256":             tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"AES256-GCM-SHA384":             tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"AES128-SHA256":                 tls.TLS_RSA_WITH_AES_128_CBC_SHA256,

		// TLS 1
		"ECDHE-ECDSA-AES128-SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"ECDHE-RSA-AES128-SHA":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"ECDHE-ECDSA-AES256-SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"ECDHE-RSA-AES256-SHA":   tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

		// SSL 3
		"AES128-SHA":   tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"AES256-SHA":   tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"DES-CBC3-SHA": tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
	for _, cipherSuite := range tls.CipherSuites() {
		idByName[cipherSuite.Name] = cipherSuite.ID
	}

	ids := []uint16{}
	for _, name := range names {
		if id, ok := idByName[name]; ok {
			ids = append(ids, id)
		}
	}

	return ids
}

func SelectCipherSuitesAndMinTLSVersion(profile *ocpconfigv1.TLSSecurityProfile) ([]string, ocpconfigv1.TLSProtocolVersion) {
	if profile == nil {
		profile = &ocpconfigv1.TLSSecurityProfile{
			Type:         ocpconfigv1.TLSProfileIntermediateType,
			Intermediate: &ocpconfigv1.IntermediateTLSProfile{},
		}
	}

	if profile.Custom != nil {
		return profile.Custom.TLSProfileSpec.Ciphers, profile.Custom.TLSProfileSpec.MinTLSVersion
	}

	return ocpconfigv1.TLSProfiles[profile.Type].Ciphers, ocpconfigv1.TLSProfiles[profile.Type].MinTLSVersion
}
