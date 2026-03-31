package middleware

import (
	"net/http"
	"time"

	"github.com/kedacore/http-add-on/interceptor/metrics"
)

// Metrics records request count and duration with bounded route identity labels.
// It creates a routeInfo in the request context before calling next, and reads
// the (potentially mutated) routeInfo after next returns to set metric labels.
type Metrics struct {
	next        http.Handler
	instruments *metrics.Instruments
}

var _ http.Handler = (*Metrics)(nil)

func NewMetrics(next http.Handler, instruments *metrics.Instruments) *Metrics {
	if instruments == nil {
		panic("instruments must not be nil")
	}
	return &Metrics{
		next:        next,
		instruments: instruments,
	}
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Pass through context so routing can report which route was matched
	info := &routeInfo{}
	r = r.WithContext(contextWithRouteInfo(r.Context(), info))

	rw := newInstrumentedResponseWriter(w)
	start := time.Now()

	defer func() {
		m.instruments.RecordRequest(r.Method, rw.statusCode, info.Name, info.Namespace, time.Since(start))
	}()

	m.next.ServeHTTP(rw, r)
}
