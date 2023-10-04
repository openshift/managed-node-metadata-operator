package metrics

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus/testutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Webhook Handlers", func() {

	var (
		testNodename = "test-nodename"
		server       *ghttp.Server
	)

	BeforeEach(func() {
		resetMetrics()
		server = ghttp.NewServer()
	})
	AfterEach(func() {
		server.Close()
	})

	Context("Service Log Sent metric", func() {
		var (
			metricHelpHeader = `
# HELP mnmo_node_reconciliation_failure Reconciliation failures occuring when updating a specific node
# TYPE mnmo_node_reconciliation_failure counter
`
			metricValueHeader = fmt.Sprintf(`mnmo_node_reconciliation_failure{node="%s"} `, testNodename)
		)
		When("the metric is set once", func() {
			It("does so correctly", func() {
				IncreaseNodeReconciliationFailure(testNodename)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 1)
				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

		When("the metric is set twice", func() {
			It("increments the metric", func() {
				IncreaseNodeReconciliationFailure(testNodename)
				IncreaseNodeReconciliationFailure(testNodename)
				expectedMetric := fmt.Sprintf("%s%s%d\n", metricHelpHeader, metricValueHeader, 2)
				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

	})
})

func resetMetrics() {
	NodeReconciliationFailure.Reset()
}
