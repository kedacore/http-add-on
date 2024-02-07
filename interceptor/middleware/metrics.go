package middleware

import (
	"net/http"

	"github.com/kedacore/http-add-on/interceptor/metrics"
)

type Metrics struct {
	upstreamHandler http.Handler
}

func NewMetrics(upstreamHandler http.Handler) *Metrics {
	return &Metrics{
		upstreamHandler: upstreamHandler,
	}
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w = newResponseWriter(w)

	defer m.metrics(w, r)

	m.upstreamHandler.ServeHTTP(w, r)
}

func (m *Metrics) metrics(w http.ResponseWriter, r *http.Request) {
	mrw := w.(*responseWriter)
	if mrw == nil {
		mrw = newResponseWriter(w)
	}

	// exclude readiness & liveness probes from the emitted metrics
	if r.URL.Path != "/livez" && r.URL.Path != "/readyz" {
		metrics.RecordRequestCount(r.Method, r.URL.Path, mrw.statusCode, r.Host)
	}
}
