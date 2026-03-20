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
	rw := newInstrumentedResponseWriter(w)

	defer m.metrics(rw, r)

	m.upstreamHandler.ServeHTTP(rw, r)
}

func (m *Metrics) metrics(rw *instrumentedResponseWriter, r *http.Request) {
	// exclude readiness & liveness probes from the emitted metrics
	if r.URL.Path != "/livez" && r.URL.Path != "/readyz" {
		metrics.RecordRequestCount(r.Method, r.URL.Path, rw.statusCode, r.Host)
	}
}
