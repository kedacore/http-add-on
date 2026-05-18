package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricsNamespace = "keda_http_addon"
	metricsSubsystem = "operator"
)

var ReconcileTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "reconcile_total",
		Help:      "Total number of reconciliations by controller and result.",
	},
	[]string{"controller", "result"},
)

func init() {
	metrics.Registry.MustRegister(
		ReconcileTotal,
	)
}
