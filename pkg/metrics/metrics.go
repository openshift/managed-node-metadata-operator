package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	NodeReconciliationFailure = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "mnmo_node_reconciliation_failure",
		Help:        "Reconciliation failures occuring when updating a specific node",
		ConstLabels: map[string]string{},
	}, []string{"node"})
)

func init() {
	metrics.Registry.MustRegister(NodeReconciliationFailure)
}
