package http

import (
	"net/http"
	"sync/atomic"
)

// drainHandler sets "Connection: close" on HTTP/1.x responses during shutdown,
// signaling clients to stop reusing the connection so in-flight requests can drain.
// HTTP/2 is not handled here because Go's HTTP/2 server already sends GOAWAY frames
// when the server is shutting down.
type drainHandler struct {
	next     http.Handler
	draining *atomic.Bool
}

var _ http.Handler = (*drainHandler)(nil)

func newDrainHandler(next http.Handler, draining *atomic.Bool) *drainHandler {
	return &drainHandler{
		next:     next,
		draining: draining,
	}
}

func (d *drainHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d.draining.Load() && r.ProtoMajor == 1 {
		w.Header().Set("Connection", "close")
	}
	d.next.ServeHTTP(w, r)
}
