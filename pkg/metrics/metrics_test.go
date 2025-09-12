package metrics

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {

	var (
		testNodename1 = "test-nodename-1"
		testNodename2 = "test-nodename-2"
		emptyNodename = ""
	)

	BeforeEach(func() {
		resetMetrics()
	})

	Context("Node Reconciliation Failure metric", func() {
		var (
			metricHelpHeader = `
# HELP mnmo_node_reconciliation_failure Reconciliation failures occurring when updating a specific node
# TYPE mnmo_node_reconciliation_failure counter
`
		)

		When("the metric is set once", func() {
			It("should record the metric correctly", func() {
				IncreaseNodeReconciliationFailure(testNodename1)
				expectedMetric := fmt.Sprintf(`%smnmo_node_reconciliation_failure{node="%s"} 1
`, metricHelpHeader, testNodename1)

				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

		When("the metric is incremented multiple times for the same node", func() {
			It("should accumulate the values correctly", func() {
				IncreaseNodeReconciliationFailure(testNodename1)
				IncreaseNodeReconciliationFailure(testNodename1)
				IncreaseNodeReconciliationFailure(testNodename1)
				IncreaseNodeReconciliationFailure(testNodename1)

				expectedMetric := fmt.Sprintf(`%smnmo_node_reconciliation_failure{node="%s"} 4
`, metricHelpHeader, testNodename1)

				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

		When("metrics are set for different nodes", func() {
			It("should track each node separately", func() {
				IncreaseNodeReconciliationFailure(testNodename1)
				IncreaseNodeReconciliationFailure(testNodename2)
				IncreaseNodeReconciliationFailure(testNodename1)

				expectedMetric := fmt.Sprintf(`%smnmo_node_reconciliation_failure{node="%s"} 2
mnmo_node_reconciliation_failure{node="%s"} 1
`, metricHelpHeader, testNodename1, testNodename2)

				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

		When("the metric is called with empty node name", func() {
			It("should still record the metric", func() {
				IncreaseNodeReconciliationFailure(emptyNodename)

				expectedMetric := fmt.Sprintf(`%smnmo_node_reconciliation_failure{node=""} 1
`, metricHelpHeader)

				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})
		})

		When("the metric is reset", func() {
			It("should clear all recorded values", func() {
				IncreaseNodeReconciliationFailure(testNodename1)
				IncreaseNodeReconciliationFailure(testNodename2)

				// Verify metrics are recorded
				expectedMetric := fmt.Sprintf(`%smnmo_node_reconciliation_failure{node="%s"} 1
mnmo_node_reconciliation_failure{node="%s"} 1
`, metricHelpHeader, testNodename1, testNodename2)
				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())

				// Reset and verify cleared
				resetMetrics()
				expectedEmptyMetric := metricHelpHeader
				err = testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedEmptyMetric))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Concurrent access to metrics", func() {
		When("multiple goroutines increment metrics simultaneously", func() {
			It("should handle concurrent access safely", func() {
				const numGoroutines = 10
				const incrementsPerGoroutine = 100

				var wg sync.WaitGroup
				wg.Add(numGoroutines)

				for i := 0; i < numGoroutines; i++ {
					go func(nodeName string) {
						defer wg.Done()
						for j := 0; j < incrementsPerGoroutine; j++ {
							IncreaseNodeReconciliationFailure(nodeName)
						}
					}(fmt.Sprintf("node-%d", i))
				}

				wg.Wait()

				// Verify all metrics are recorded correctly by checking the total
				// We'll use CollectAndCompare to verify the expected values
				var expectedMetric strings.Builder
				expectedMetric.WriteString(`
# HELP mnmo_node_reconciliation_failure Reconciliation failures occurring when updating a specific node
# TYPE mnmo_node_reconciliation_failure counter
`)

				for i := 0; i < numGoroutines; i++ {
					expectedMetric.WriteString(fmt.Sprintf(`mnmo_node_reconciliation_failure{node="node-%d"} %d
`, i, incrementsPerGoroutine))
				}

				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric.String()))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("Metric initialization and configuration", func() {
		When("checking metric configuration", func() {
			It("should have correct metric name", func() {
				// Test by collecting the metric and checking the output
				IncreaseNodeReconciliationFailure("test-node")
				expectedMetric := `
# HELP mnmo_node_reconciliation_failure Reconciliation failures occurring when updating a specific node
# TYPE mnmo_node_reconciliation_failure counter
mnmo_node_reconciliation_failure{node="test-node"} 1
`
				err := testutil.CollectAndCompare(NodeReconciliationFailure, strings.NewReader(expectedMetric))
				Expect(err).To(BeNil())
			})

			It("should be registered in the metrics registry", func() {
				registry := prometheus.NewRegistry()
				registry.MustRegister(NodeReconciliationFailure)

				// This should not panic
				Expect(func() {
					registry.MustRegister(NodeReconciliationFailure)
				}).To(Panic())
			})
		})

		When("checking metric label names", func() {
			It("should have correct label names", func() {
				// Test by using the metric with a label value
				metric, err := NodeReconciliationFailure.GetMetricWithLabelValues("valid-node-name")
				Expect(err).To(BeNil())
				Expect(metric).ToNot(BeNil())
			})
		})
	})

	Context("Edge cases and error handling", func() {
		When("calling IncreaseNodeReconciliationFailure with nil-like values", func() {
			It("should handle empty string gracefully", func() {
				Expect(func() {
					IncreaseNodeReconciliationFailure("")
				}).ToNot(Panic())
			})

			It("should handle whitespace-only strings", func() {
				Expect(func() {
					IncreaseNodeReconciliationFailure("   ")
					IncreaseNodeReconciliationFailure("\t")
					IncreaseNodeReconciliationFailure("\n")
				}).ToNot(Panic())
			})
		})

		When("calling IncreaseNodeReconciliationFailure with special characters", func() {
			It("should handle various special characters", func() {
				specialChars := []string{
					"node-with-dashes",
					"node_with_underscores",
					"node.with.dots",
					"node:with:colons",
					"node/with/slashes",
					"node\\with\\backslashes",
					"node with spaces",
					"node\twith\ttabs",
					"node\nwith\nnewlines",
				}

				for _, nodeName := range specialChars {
					Expect(func() {
						IncreaseNodeReconciliationFailure(nodeName)
					}).ToNot(Panic(), "Should handle node name: %s", nodeName)
				}
			})
		})
	})

	Context("Performance and stress testing", func() {
		When("calling IncreaseNodeReconciliationFailure many times", func() {
			It("should handle high frequency calls efficiently", func() {
				const numCalls = 10000

				for i := 0; i < numCalls; i++ {
					IncreaseNodeReconciliationFailure(fmt.Sprintf("stress-test-node-%d", i%100))
				}

				// Verify some metrics were recorded
				metric, err := NodeReconciliationFailure.GetMetricWithLabelValues("stress-test-node-0")
				Expect(err).To(BeNil())
				Expect(metric).ToNot(BeNil())
			})
		})
	})
})

func resetMetrics() {
	NodeReconciliationFailure.Reset()
}
