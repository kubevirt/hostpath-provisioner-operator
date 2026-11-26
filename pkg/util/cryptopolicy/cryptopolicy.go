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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("cryptopolicy")

type cryptoConfig struct {
	CipherSuites     []uint16
	CurvePreferences []tls.CurveID
	MinVersion       uint16
}

// TLSArgs holds TLS settings as strings, ready to pass as command-line flags
// to the CSI plugin's metrics server. Lists are comma-separated; an empty
// field leaves Go's default in place.
type TLSArgs struct {
	CipherSuites string
	Groups       string
	MinVersion   string
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
	profileSpec := getTLSProfileSpec(nil)
	return &ManagedTLSWatcher{
		defaultConfig: &cryptoConfig{
			CipherSuites:     cipherSuitesIDs(profileSpec.Ciphers),
			CurvePreferences: groupsToCurves(profileSpec.Groups),
			MinVersion:       getTLSVersionOrDefault(string(profileSpec.MinTLSVersion)),
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

func tlsOverrides() *TLSArgs {
	tlsArgs := &TLSArgs{
		CipherSuites: os.Getenv("TLS_CIPHERS_OVERRIDE"),
		MinVersion:   os.Getenv("TLS_MIN_VERSION_OVERRIDE"),
		Groups:       os.Getenv("TLS_GROUPS_OVERRIDE"),
	}
	if tlsArgs.CipherSuites == "" && tlsArgs.MinVersion == "" && tlsArgs.Groups == "" {
		return nil
	}
	return tlsArgs
}

// ResolveTLSArgs returns the TLS settings for the CSI plugin's metrics server.
// Env var overrides take precedence, then the cluster APIServer's TLS profile,
// then the Intermediate profile when no APIServer CR is available.
func ResolveTLSArgs(reader client.Reader) *TLSArgs {
	if override := tlsOverrides(); override != nil {
		return override
	}
	var profile *ocpconfigv1.TLSSecurityProfile
	apiServer := &ocpconfigv1.APIServer{}
	if err := reader.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, apiServer); err == nil {
		profile = apiServer.Spec.TLSSecurityProfile
	}
	spec := getTLSProfileSpec(profile)
	return &TLSArgs{
		CipherSuites: strings.Join(spec.Ciphers, ","),
		MinVersion:   string(spec.MinTLSVersion),
		Groups:       joinGroups(spec.Groups),
	}
}

func joinGroups(items []ocpconfigv1.TLSGroup) string {
	s := make([]string, 0, len(items))
	for _, v := range items {
		s = append(s, string(v))
	}
	return strings.Join(s, ",")
}

// getTLSConfig returns the current TLS configuration. Override env vars take
// precedence, then the APIServer CR from cache, then the default Intermediate profile.
func (m *ManagedTLSWatcher) getTLSConfig(ctx context.Context) *cryptoConfig {
	if overrides := tlsOverrides(); overrides != nil {
		cc := &cryptoConfig{}
		if overrides.CipherSuites != "" {
			cc.CipherSuites = cipherSuitesIDs(strings.Split(overrides.CipherSuites, ","))
		}
		if overrides.MinVersion != "" {
			cc.MinVersion = getTLSVersionOrDefault(overrides.MinVersion)
		}
		if overrides.Groups != "" {
			var tlsGroups []ocpconfigv1.TLSGroup
			for group := range strings.SplitSeq(overrides.Groups, ",") {
				tlsGroups = append(tlsGroups, ocpconfigv1.TLSGroup(group))
			}
			cc.CurvePreferences = groupsToCurves(tlsGroups)
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
	profileSpec := getTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
	return &cryptoConfig{
		CipherSuites:     cipherSuitesIDs(profileSpec.Ciphers),
		CurvePreferences: groupsToCurves(profileSpec.Groups),
		MinVersion:       getTLSVersionOrDefault(string(profileSpec.MinTLSVersion)),
	}
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
			cc := m.getTLSConfig(hello.Context())
			config.CipherSuites = cc.CipherSuites
			config.CurvePreferences = cc.CurvePreferences
			config.MinVersion = cc.MinVersion
			return config, nil
		}
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

	ids := make([]uint16, 0, len(names))
	for _, name := range names {
		if id, ok := idByName[name]; ok {
			ids = append(ids, id)
		}
	}

	return ids
}

func getTLSProfileSpec(profile *ocpconfigv1.TLSSecurityProfile) ocpconfigv1.TLSProfileSpec {
	if profile == nil {
		return *ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType]
	}
	if profile.Custom != nil {
		return profile.Custom.TLSProfileSpec
	}
	if spec, ok := ocpconfigv1.TLSProfiles[profile.Type]; ok {
		return *spec
	}
	log.V(1).Info("invalid TLS security profile, falling back to Intermediate", "type", profile.Type)
	return *ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType]
}

// groupsToCurves translates groups names to CurveID which can go straight to the tls.Config
func groupsToCurves(names []ocpconfigv1.TLSGroup) []tls.CurveID {
	// ref: https://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-8
	var idByName = map[ocpconfigv1.TLSGroup]tls.CurveID{
		ocpconfigv1.TLSGroupX25519:             tls.X25519,
		ocpconfigv1.TLSGroupSecP256r1:          tls.CurveP256,
		ocpconfigv1.TLSGroupSecP384r1:          tls.CurveP384,
		ocpconfigv1.TLSGroupSecP521r1:          tls.CurveP521,
		ocpconfigv1.TLSGroupX25519MLKEM768:     tls.X25519MLKEM768,
		ocpconfigv1.TLSGroupSecP256r1MLKEM768:  tls.SecP256r1MLKEM768,
		ocpconfigv1.TLSGroupSecP384r1MLKEM1024: tls.SecP384r1MLKEM1024,
	}

	ids := make([]tls.CurveID, 0, len(names))
	for _, name := range names {
		if id, ok := idByName[name]; ok {
			ids = append(ids, id)
		}
	}

	return ids
}
