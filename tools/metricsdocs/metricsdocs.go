package main

import (
	"fmt"

	"github.com/machadovilaca/operator-observability/pkg/docs"
	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/metrics"
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

### kubevirt_hpp_operator_up
The number of running hostpath-provisioner-operator pods. Type: Gauge.

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
`

func main() {
	err := metrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	metricsList := metrics.ListMetrics()

	docsString := docs.BuildMetricsDocsWithCustomTemplate(metricsList, nil, tpl)
	fmt.Print(docsString)
}
