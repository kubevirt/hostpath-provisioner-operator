package main

import (
	"github.com/kubevirt/monitoring/pkg/metrics/parser"
	"kubevirt.io/hostpath-provisioner-operator/pkg/controller/hostpathprovisioner"

	dto "github.com/prometheus/client_model/go"
)

// excludedMetrics defines the metrics to ignore,
// open bug: https://bugzilla.redhat.com/show_bug.cgi?id=2219763
// Do not add metrics to this list!
var excludedMetrics = map[string]struct{}{
	"kubevirt_hpp_operator_up_total": {},
}

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
