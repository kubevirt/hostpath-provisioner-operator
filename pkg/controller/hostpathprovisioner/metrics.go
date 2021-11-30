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

package hostpathprovisioner

import (
	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logc "log"
	"net/http"
)

// Handler creates a new prometheus handler to receive scrap requests
func Handler(MaxRequestsInFlight int) http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				MaxRequestsInFlight: MaxRequestsInFlight,
			}),
	)
}

// NewPrometheusScraper returns a new struct of the prometheus scrapper
func NewPrometheusScraper(ch chan<- prometheus.Metric) *PrometheusScraper {
	return &PrometheusScraper{ch: ch}
}

// PrometheusScraper struct containing the resources to scrap prometheus metrics
type PrometheusScraper struct {
	ch chan<- prometheus.Metric
}

// Report adds CDI metrics to PrometheusScraper
func (ps *PrometheusScraper) Report(socketFile string) {
	defer func() {
		if err := recover(); err != nil {
			logc.Panicf("collector goroutine panicked for VM %s: %s", socketFile, err)
		}
	}()

	for _, rule := range createPrometheusRules("-") {
		if rule.Record != "" {
			fqName := rule.Record
			help := getSummary(rule)
			labels := getLabels(rule)
			ps.newMetric(fqName, help, labels)
		}
	}
}

func getSummary(rule promv1.Rule) string {
	if rule.Annotations == nil {
		return ""
	}

	summary, found := rule.Annotations["summary"]

	if !found {
		return ""
	}

	return summary
}

func getLabels(rule promv1.Rule) []string {
	if rule.Labels == nil {
		return make([]string, 0)
	}

	keys := make([]string, len(rule.Labels))

	i := 0
	for k := range rule.Labels {
		keys[i] = k
		i++
	}

	return keys
}

func (ps *PrometheusScraper) newMetric(fqName string, help string, constLabelPairs []string) {
	desc := prometheus.NewDesc(
		fqName,
		help,
		constLabelPairs,
		nil,
	)

	mv, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 1024, constLabelPairs...)
	if err != nil {
		panic(err)
	}
	ps.ch <- mv
}
