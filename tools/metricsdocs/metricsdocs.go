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

	"github.com/machadovilaca/operator-observability/pkg/docs"

	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/metrics"
	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules"
)

const tpl = `# Hostpath Provisioner Operator Metrics
{{- range . }}

{{ $deprecatedVersion := "" -}}
{{- with index .ExtraFields "DeprecatedVersion" -}}
    {{- $deprecatedVersion = printf " in %s" . -}}
{{- end -}}

{{- $stabilityLevel := "" -}}
{{- if and (.ExtraFields.StabilityLevel) (ne .ExtraFields.StabilityLevel "STABLE") -}}
	{{- $stabilityLevel = printf "[%s%s] " .ExtraFields.StabilityLevel $deprecatedVersion -}}
{{- end -}}

### {{ .Name }}
{{ print $stabilityLevel }}{{ .Help }}. Type: {{ .Type -}}.

{{- end }}

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
`

func main() {
	if err := metrics.SetupMetrics(); err != nil {
		panic(err)
	}

	if err := rules.SetupRules("test"); err != nil {
		panic(err)
	}

	docsString := docs.BuildMetricsDocsWithCustomTemplate(metrics.ListMetrics(), rules.ListRecordingRules(), tpl)
	fmt.Print(docsString)
}
