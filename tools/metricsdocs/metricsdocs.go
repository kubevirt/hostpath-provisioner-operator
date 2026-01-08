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

// Package main is the entry point for the Hostpath Provisioner Operator's metrics documentation tool.
// This tool generates documentation for metrics and rules used in the Hostpath Provisioner Operator,
package main

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/docs"

	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/metrics"
	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules"
)

const title = `Hostpath Provisioner Operator Metrics`

func main() {
	if err := metrics.SetupMetrics(); err != nil {
		panic(err)
	}

	if err := rules.SetupRules("test"); err != nil {
		panic(err)
	}

	docsString := docs.BuildMetricsDocs(title, metrics.ListMetrics(), rules.ListRecordingRules())
	fmt.Print(docsString)
}
