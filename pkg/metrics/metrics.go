package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	DefaultMachineSetMetricsAddress = ":8082"
	DefaultMachineMetricsAddress    = ":8081"
	DefaultNodeMetricsAddress       = ":8083"
)

var (
	// MnmoCollectorUp is a Prometheus metric, which reports reflects successful collection and reporting of all the metrics
	MnmoCollectorUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mnmo_collector_up",
		Help: "Managed node metadata Operator metrics are being collected and reported successfully",
	}, []string{"kind"})

	failedLabelUpdateCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mnmo_label_update_failed",
			Help: "Number of times label update has failed.",
		}, []string{"name", "namespace", "reason"},
	)

	failedTaintUpdateCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mnmo_taint_update_failed",
			Help: "Number of times taint update has failed.",
		}, []string{"name", "namespace", "reason"},
	)
)

func init() {
	prometheus.MustRegister(MnmoCollectorUp)
	metrics.Registry.MustRegister(
		failedLabelUpdateCount,
		failedTaintUpdateCount,
	)
}

// MnmoLabels is the group of labels that are applied to the mnmo metrics
type MnmoLabels struct {
	Name      string
	Namespace string
	Reason    string
}

func RegisterFailedLabelUpdate(labels *MnmoLabels) {
	failedLabelUpdateCount.With(prometheus.Labels{
		"name":      labels.Name,
		"namespace": labels.Namespace,
		"reason":    labels.Reason,
	}).Inc()
}

func RegisterFailedTaintUpdate(labels *MnmoLabels) {
	failedTaintUpdateCount.With(prometheus.Labels{
		"name":      labels.Name,
		"namespace": labels.Namespace,
		"reason":    labels.Reason,
	}).Inc()
}
