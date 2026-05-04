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

// Package cryptopolicy contains function to manage the crypto policy
package cryptopolicy

import (
	"crypto/tls"
	"os"
	"strings"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/klog/v2"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// GetWebhookServerSpec sets the GetConfigForClient to always check for ciphers and minimum TLS version
func GetWebhookServerSpec() webhook.Server {
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

	return &webhook.DefaultServer{
		Options: webhook.Options{
			TLSOpts: []func(*tls.Config){
				tlsCfgMutateFunc,
			},
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

// GetTLSMinVersionString returns the TLS minimum version string to use based on environment variables.
// It checks environment variables in this order:
//  1. TLS_MIN_VERSION_OVERRIDE (manual override)
//  2. TLS_MIN_VERSION (set by APIServer watch on OpenShift)
//  3. Default: VersionTLS13
//
// Returns a string like "VersionTLS13" suitable for passing to command-line arguments.
// This ensures consistent precedence logic across the operator and daemonset configuration.
// If an invalid value is provided, it logs a warning and returns the default.
//
// Note: TLS_MIN_VERSION is set by handleAPIServerFunc when watching APIServer resources,
// which may not have run yet during the first DaemonSet reconcile. In that case, the
// DaemonSet will initially be created with the default (VersionTLS13), and will be
// updated on a subsequent reconcile after the APIServer watch fires. This is normal
// self-healing behavior and ensures secure defaults.
func GetTLSMinVersionString() string {
	validVersions := map[string]bool{
		"VersionTLS10": true,
		"VersionTLS11": true,
		"VersionTLS12": true,
		"VersionTLS13": true,
	}

	// Check for override first
	if tlsVersion := os.Getenv("TLS_MIN_VERSION_OVERRIDE"); tlsVersion != "" {
		if !validVersions[tlsVersion] {
			klog.Warningf("Invalid TLS_MIN_VERSION_OVERRIDE value '%s', defaulting to VersionTLS13. Valid values: VersionTLS10, VersionTLS11, VersionTLS12, VersionTLS13", tlsVersion)
			return "VersionTLS13"
		}
		return tlsVersion
	}

	// Then fall back to cluster settings
	if tlsVersion := os.Getenv("TLS_MIN_VERSION"); tlsVersion != "" {
		if !validVersions[tlsVersion] {
			klog.Warningf("Invalid TLS_MIN_VERSION value '%s', defaulting to VersionTLS13. Valid values: VersionTLS10, VersionTLS11, VersionTLS12, VersionTLS13", tlsVersion)
			return "VersionTLS13"
		}
		return tlsVersion
	}

	// Default to TLS 1.3 if no environment variable is set
	return "VersionTLS13"
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

// SelectCipherSuitesAndMinTLSVersion selects the cipher suite and minimum TLS version based on the passed in security profile
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

// GetMetricsServerOptions returns metrics server options configured for secure serving.
// The TLS configuration dynamically reads from environment variables on each client connection.
// Environment variables are updated by the APIServer resource watch (see handleAPIServerFunc in controller.go):
// - TLS_CIPHERS: Comma-separated cipher suite names from cluster TLS profile
// - TLS_MIN_VERSION: Minimum TLS version from cluster TLS profile (e.g., "VersionTLS13")
// - TLS_CIPHERS_OVERRIDE: Overrides cluster cipher suites (takes precedence)
// - TLS_MIN_VERSION_OVERRIDE: Overrides cluster TLS version (takes precedence)
//
// Override variables are useful for non-OpenShift clusters or to deviate from cluster-wide settings.
func GetMetricsServerOptions() metricsserver.Options {
	tlsCfgMutateFunc := func(cfg *tls.Config) {
		// This callback executes on each client call returning a new config to be used.
		// It dynamically reads the environment variables which are updated when the
		// cluster's APIServer TLSSecurityProfile changes.
		// Clone the config to avoid concurrent modification issues.
		cfg.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
			newCfg := cfg.Clone()

			tlsCiphersOverride := os.Getenv("TLS_CIPHERS_OVERRIDE")
			// Check for override first, then fall back to cluster settings
			if tlsCiphersOverride != "" {
				// Use override ciphers
				ciphersNames := strings.Split(tlsCiphersOverride, ",")
				ciphers := cipherSuitesIDs(ciphersNames)
				if len(ciphers) != 0 {
					newCfg.CipherSuites = ciphers
				}
			} else {
				// Use cluster TLS profile ciphers
				ciphersNames := strings.Split(os.Getenv("TLS_CIPHERS"), ",")
				ciphers := cipherSuitesIDs(ciphersNames)
				if len(ciphers) != 0 {
					newCfg.CipherSuites = ciphers
				}
			}

			tlsMinVersionOverride := os.Getenv("TLS_MIN_VERSION_OVERRIDE")
			if tlsMinVersionOverride != "" {
				// Use override min version
				overrideMinVersion := getTLSVersion(tlsMinVersionOverride)
				if overrideMinVersion != nil {
					newCfg.MinVersion = *overrideMinVersion
				}
			} else {
				// Use cluster TLS profile min version
				clusterMinVersion := getTLSVersion(os.Getenv("TLS_MIN_VERSION"))
				if clusterMinVersion != nil {
					newCfg.MinVersion = *clusterMinVersion
				}
			}

			return newCfg, nil
		}
	}

	return metricsserver.Options{
		SecureServing: true,
		BindAddress:   ":8443",
		TLSOpts: []func(*tls.Config){
			tlsCfgMutateFunc,
		},
	}
}
