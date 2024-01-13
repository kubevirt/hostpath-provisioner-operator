/*
Copyright 2024 The hostpath provisioner operator Authors.

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

// Package rules provides functions for setting up and managing Prometheus alerts and recording rules.
package rules

import (
	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules/alerts"
	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules/recordingrules"
	"kubevirt.io/hostpath-provisioner-operator/pkg/util"
)

const (
	hppPrometheusRuleName = "prometheus-hpp-rules"
	prometheusLabelKey    = "prometheus.hostpathprovisioner.kubevirt.io"
	prometheusLabelValue  = "true"
)

// SetupRules sets up recording and alerting rules in the specified namespace.
func SetupRules(namespace string) error {
	err := recordingrules.Register(namespace)
	if err != nil {
		return err
	}

	err = alerts.Register()
	if err != nil {
		return err
	}

	return nil
}

// BuildPrometheusRule creates a PrometheusRule resource for monitoring.
func BuildPrometheusRule(namespace string) (*promv1.PrometheusRule, error) {
	labels := util.GetRecommendedLabels()
	labels[prometheusLabelKey] = prometheusLabelValue

	return operatorrules.BuildPrometheusRule(
		hppPrometheusRuleName,
		namespace,
		labels,
	)
}

// ListRecordingRules lists all configured recording rules.
func ListRecordingRules() []operatorrules.RecordingRule {
	// Retrieve the list of recording rules
	return operatorrules.ListRecordingRules()
}

// ListAlerts lists all configured alerting rules.
func ListAlerts() []promv1.Rule {
	// Retrieve the list of alerting rules
	return operatorrules.ListAlerts()
}
