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

package recordingrules

import (
	"fmt"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func operatorRecordingRules(namespace string) []operatorrules.RecordingRule {
	return []operatorrules.RecordingRule{
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_hpp_operator_up",
				Help: "The number of running hostpath-provisioner-operator pods",
			},
			MetricType: operatormetrics.GaugeType,
			Expr:       intstr.FromString(fmt.Sprintf("sum(up{namespace='%s', pod=~'hostpath-provisioner-operator-.*'} or vector(0))", namespace)),
		},
	}
}
