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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

var log = ctrl.Log.WithName("cryptopolicy")

type cryptoConfig struct {
	CipherSuites []uint16
	MinVersion   uint16
}

// ManagedTLSWatcher reads TLS configuration from the cluster's APIServer CR
// via the informer cache, eliminating the need for environment variable exchange.
type ManagedTLSWatcher struct {
	mu            sync.RWMutex
	cache         cache.Cache
	defaultConfig *cryptoConfig
	ready         bool
}

// NewManagedTLSWatcher creates a new ManagedTLSWatcher with Intermediate profile defaults.
func NewManagedTLSWatcher() *ManagedTLSWatcher {
	defaultProfile := &ocpconfigv1.TLSSecurityProfile{
		Type:         ocpconfigv1.TLSProfileIntermediateType,
		Intermediate: &ocpconfigv1.IntermediateTLSProfile{},
	}
	cipherNames, minVersion := selectCipherSuitesAndMinTLSVersion(defaultProfile)
	return &ManagedTLSWatcher{
		defaultConfig: &cryptoConfig{
			CipherSuites: cipherSuitesIDs(cipherNames),
			MinVersion:   getTLSVersionOrDefault(string(minVersion)),
		},
	}
}

// SetCache injects the controller-runtime cache after manager creation.
func (m *ManagedTLSWatcher) SetCache(c cache.Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = c
}

// Start implements manager.Runnable. It waits for the cache to sync, then
// marks the watcher as ready.
func (m *ManagedTLSWatcher) Start(ctx context.Context) error {
	m.mu.RLock()
	c := m.cache
	m.mu.RUnlock()

	if c == nil {
		return fmt.Errorf("no cache provided for tls watcher")
	}
	log.Info("ManagedTLSWatcher: starting, waiting for cache sync")
	if !c.WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	m.mu.Lock()
	m.ready = true
	m.mu.Unlock()
	log.Info("ManagedTLSWatcher: ready")

	<-ctx.Done()
	return nil
}

// NeedLeaderElection implements manager.LeaderElectionRunnable.
func (m *ManagedTLSWatcher) NeedLeaderElection() bool {
	return false
}

// GetTLSConfig returns the current TLS configuration. Override env vars take
// precedence, then the APIServer CR from cache, then the default Intermediate profile.
func (m *ManagedTLSWatcher) GetTLSConfig(ctx context.Context) *cryptoConfig {
	// Override env vars always take precedence
	ciphersOverride := os.Getenv("TLS_CIPHERS_OVERRIDE")
	versionOverride := os.Getenv("TLS_MIN_VERSION_OVERRIDE")
	if ciphersOverride != "" || versionOverride != "" {
		cc := &cryptoConfig{}
		if ciphersOverride != "" {
			cc.CipherSuites = cipherSuitesIDs(strings.Split(ciphersOverride, ","))
		}
		if versionOverride != "" {
			cc.MinVersion = getTLSVersionOrDefault(versionOverride)
		}
		return cc
	}

	m.mu.RLock()
	ready := m.ready
	c := m.cache
	m.mu.RUnlock()

	if !ready || c == nil {
		return m.defaultConfig
	}

	apiServer := &ocpconfigv1.APIServer{}
	if err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, apiServer); err != nil {
		return m.defaultConfig
	}

	return cryptoConfigFromProfile(apiServer.Spec.TLSSecurityProfile)
}

// CryptoPolicyOpt returns a TLS config mutator that dynamically applies
// the current crypto policy on each TLS handshake.
func (m *ManagedTLSWatcher) CryptoPolicyOpt() func(*tls.Config) {
	return func(c *tls.Config) {
		// Disable HTTP/2 to prevent rapid reset vulnerability
		// See CVE-2023-44487, CVE-2023-39325
		c.NextProtos = []string{"http/1.1"}
		c.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			config := c.Clone()
			cc := m.GetTLSConfig(hello.Context())
			config.CipherSuites = cc.CipherSuites
			config.MinVersion = cc.MinVersion
			return config, nil
		}
	}
}

func cryptoConfigFromProfile(profile *ocpconfigv1.TLSSecurityProfile) *cryptoConfig {
	cipherNames, minVersion := selectCipherSuitesAndMinTLSVersion(profile)
	return &cryptoConfig{
		CipherSuites: cipherSuitesIDs(cipherNames),
		MinVersion:   getTLSVersionOrDefault(string(minVersion)),
	}
}

func getTLSVersionOrDefault(versionName string) uint16 {
	if v := getTLSVersion(versionName); v != nil {
		return *v
	}
	return tls.VersionTLS13
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
	// Check for override first
	if tlsVersion := os.Getenv("TLS_MIN_VERSION_OVERRIDE"); tlsVersion != "" {
		// Validate using existing getTLSVersion function
		if getTLSVersion(tlsVersion) == nil {
			klog.Warningf("Invalid TLS_MIN_VERSION_OVERRIDE value '%s', defaulting to VersionTLS13. Valid values: VersionTLS10, VersionTLS11, VersionTLS12, VersionTLS13", tlsVersion)
			return "VersionTLS13"
		}
		return tlsVersion
	}

	// Then fall back to cluster settings
	if tlsVersion := os.Getenv("TLS_MIN_VERSION"); tlsVersion != "" {
		// Validate using existing getTLSVersion function
		if getTLSVersion(tlsVersion) == nil {
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

func selectCipherSuitesAndMinTLSVersion(profile *ocpconfigv1.TLSSecurityProfile) ([]string, ocpconfigv1.TLSProtocolVersion) {
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
