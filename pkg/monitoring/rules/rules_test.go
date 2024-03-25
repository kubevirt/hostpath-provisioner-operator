package rules

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/machadovilaca/operator-observability/pkg/testutil"
)

func TestRules(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Rules Suite")
}

var _ = ginkgo.Describe("Rules Validation", func() {
	var linter *testutil.Linter

	ginkgo.BeforeEach(func() {
		gomega.Expect(SetupRules("")).To(gomega.Succeed())
		linter = testutil.New()
	})

	ginkgo.It("Should validate alerts", func() {
		linter.AddCustomAlertValidations(
			testutil.ValidateAlertNameLength,
			testutil.ValidateAlertRunbookURLAnnotation,
			testutil.ValidateAlertHealthImpactLabel,
			testutil.ValidateAlertPartOfAndComponentLabels)

		alerts := ListAlerts()
		problems := linter.LintAlerts(alerts)
		gomega.Expect(problems).To(gomega.BeEmpty())
	})

	ginkgo.It("Should validate recording rules", func() {
		recordingRules := ListRecordingRules()
		problems := linter.LintRecordingRules(recordingRules)
		gomega.Expect(problems).To(gomega.BeEmpty())
	})
})
