package main

import (
	"github.com/kubevirt/monitoring/pkg/metrics/parser"
	"kubevirt.io/hostpath-provisioner-operator/pkg/controller/hostpathprovisioner"

	dto "github.com/prometheus/client_model/go"
)

// This should be used only for very rare cases where the naming conventions that are explained in the best practices:
// https://sdk.operatorframework.io/docs/best-practices/observability-best-practices/#metrics-guidelines
// should be ignored.
var excludedMetrics = map[string]struct{}{}

func readMetrics() []*dto.MetricFamily {
	var metricFamilies []*dto.MetricFamily
	hppMetrics := hostpathprovisioner.GetRecordRulesDesc("")

	for _, metric := range hppMetrics {
		if _, isExcludedMetric := excludedMetrics[metric.Name]; !isExcludedMetric {
			mf := parser.CreateMetricFamily(parser.Metric{
				Name: metric.Name,
				Help: metric.Description,
				Type: metric.Type,
			})
			metricFamilies = append(metricFamilies, mf)
		}
	}

	return metricFamilies
}
