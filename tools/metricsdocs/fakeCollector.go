package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"kubevirt.io/hostpath-provisioner-operator/pkg/controller/hostpathprovisioner"
)

type fakeCollector struct {
}

func (fc fakeCollector) Describe(_ chan<- *prometheus.Desc) {
}

//Collect needs to report all metrics to see it in docs
func (fc fakeCollector) Collect(ch chan<- prometheus.Metric) {
	ps := hostpathprovisioner.NewPrometheusScraper(ch)
	ps.Report("test")
}

func RegisterFakeCollector() {
	prometheus.MustRegister(fakeCollector{})
}
