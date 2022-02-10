package hostpathprovisioner

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("HPP alerts", func() {
	Context("runbooks", func() {
		It("Should have available URLs", func() {
			for _, rule := range getAlertRules() {
				Expect(rule.Annotations).ToNot(BeNil())
				url, ok := rule.Annotations["runbook_url"]
				Expect(ok).To(BeTrue())
				resp, err := http.Head(url)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			}
		})
	})
})
