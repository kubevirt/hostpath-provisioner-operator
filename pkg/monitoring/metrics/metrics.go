/*
Copyright 2019 The hostpath provisioner operator Authors.

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

// Package metrics is the main prometheus metrics package
package metrics

import (
	"github.com/machadovilaca/operator-observability/pkg/operatormetrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// SetupMetrics register the metrics
func SetupMetrics() error {
	operatormetrics.Register = metrics.Registry.Register

	return operatormetrics.RegisterMetrics(
		operatorMetrics,
	)
}

// ListMetrics list the metrics opts
func ListMetrics() []operatormetrics.Metric {
	return operatormetrics.ListMetrics()
}
