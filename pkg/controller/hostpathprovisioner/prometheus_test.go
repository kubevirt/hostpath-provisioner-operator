package hostpathprovisioner

import (
	"fmt"
	"os"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"kubevirt.io/hostpath-provisioner-operator/pkg/monitoring/rules"
)

var _ = ginkgo.Describe("Prometheus", func() {
	ginkgo.BeforeEach(func() {
		os.Unsetenv(runbookURLTemplateEnv)
	})

	ginkgo.AfterEach(func() {
		os.Unsetenv(runbookURLTemplateEnv)
	})

	ginkgo.It("should use the default runbook URL template when no ENV Variable is set", func() {
		promRule, err := rules.BuildPrometheusRule("mynamespace")
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		for _, rule := range promRule.Spec.Groups[1].Rules {
			if rule.Alert != "" {
				if rule.Annotations["runbook_url"] != "" {
					gomega.Expect(rule.Annotations["runbook_url"]).To(gomega.Equal(fmt.Sprintf(defaultRunbookURLTemplate, rule.Alert)))
				}
			}
		}
	})

	ginkgo.It("should use the desired runbook URL template when its ENV Variable is set", func() {
		desiredRunbookURLTemplate := "desired/runbookURL/template/%s"
		os.Setenv(runbookURLTemplateEnv, desiredRunbookURLTemplate)

		promRule, err := rules.BuildPrometheusRule("mynamespace")
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		for _, rule := range promRule.Spec.Groups[0].Rules {
			if rule.Alert != "" {
				if rule.Annotations["runbook_url"] != "" {
					gomega.Expect(rule.Annotations["runbook_url"]).To(gomega.Equal(fmt.Sprintf(desiredRunbookURLTemplate, rule.Alert)))
				}
			}
		}
	})
})
