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

// Package alerts provides functions for setting up and managing Prometheus alerts rules.
package alerts

import (
	"errors"
	"fmt"
	"os"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
)

const (
	prometheusRunbookAnnotationKey = "runbook_url"
	defaultRunbookURLTemplate      = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv          = "RUNBOOK_URL_TEMPLATE"

	severityAlertLabelKey     = "severity"
	healthImpactAlertLabelKey = "operator_health_impact"

	partOfAlertLabelKey      = "kubernetes_operator_part_of"
	partOfAlertLabelValue    = "kubevirt"
	componentAlertLabelKey   = "kubernetes_operator_component"
	componentAlertLabelValue = "hostpath-provisioner-operator"
)

// Register initializes and registers alert rules by setting labels and annotations, including the runbook URL.
func Register() error {
	alerts := [][]promv1.Rule{
		operatorAlerts,
	}

	runbookURLTemplate := GetRunbookURLTemplate()
	for _, alertGroup := range alerts {
		for _, alert := range alertGroup {
			alert.Labels[partOfAlertLabelKey] = partOfAlertLabelValue
			alert.Labels[componentAlertLabelKey] = componentAlertLabelValue

			alert.Annotations[prometheusRunbookAnnotationKey] = fmt.Sprintf(runbookURLTemplate, alert.Alert)
		}
	}

	return operatorrules.RegisterAlerts(alerts...)
}

// GetRunbookURLTemplate retrieves the runbook URL template from environment variables or uses a default.
func GetRunbookURLTemplate() string {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		panic(errors.New("runbook URL template must have exactly 1 %s substring"))
	}

	return runbookURLTemplate
}
