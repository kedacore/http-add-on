//go:build e2e

package helpers

// InterceptorMetrics is the set of Prometheus metric names that the interceptor
// is expected to expose. Shared between the prometheus and otel metric tests.
var InterceptorMetrics = []string{
	"interceptor_pending_requests",
	"interceptor_request_duration_seconds",
	"interceptor_requests_total",
}
