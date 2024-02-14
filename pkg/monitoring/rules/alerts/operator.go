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

package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	operatorAlerts = []promv1.Rule{
		{
			Alert: "HPPOperatorDown",
			Expr:  intstr.FromString("kubevirt_hpp_operator_up == 0"),
			For:   ptr.To(promv1.Duration("5m")),
			Annotations: map[string]string{
				"summary": "Hostpath Provisioner operator is down.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "HPPNotReady",
			Expr:  intstr.FromString("kubevirt_hpp_cr_ready == 0"),
			For:   ptr.To(promv1.Duration("5m")),
			Annotations: map[string]string{
				"summary": "Hostpath Provisioner is not available to use.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: "HPPSharingPoolPathWithOS",
			Expr:  intstr.FromString("kubevirt_hpp_pool_path_shared_with_os == 1"),
			For:   ptr.To(promv1.Duration("1m")),
			Annotations: map[string]string{
				"summary": "HPP pool path sharing a filesystem with OS, fix to prevent HPP PVs from causing disk pressure and affecting node operation.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
			},
		},
	}
)
