package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// MnmoCollectorUp is a Prometheus metric, which reports reflects successful collection and reporting of all the metrics
	MnmoCollectorUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mnmo_collector_up",
		Help: "Managed node metadata Operator metrics are being collected and reported successfully",
	}, []string{"kind"})

	failedReconcileCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mnmo_reconcile_failed",
			Help: "Number of times reconcile has failed.",
		}, []string{"name", "namespace"},
	)

	successReconcileCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mnmo_reconcile_success",
			Help: "Number of times reconcile succeeded.",
		}, []string{"name", "namespace"},
	)
)

func init() {
	prometheus.MustRegister(MnmoCollectorUp)
	metrics.Registry.MustRegister(
		failedReconcileCount,
		successReconcileCount,
	)
}

func RegisterFailedReconcile() {
	failedReconcileCount.With(prometheus.Labels{
		"success": "false",
	}).Inc()
}

func RegisterSuccessReconcile() {
	successReconcileCount.With(prometheus.Labels{
		"success": "true",
	}).Inc()
}
