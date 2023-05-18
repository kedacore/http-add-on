package main

import (
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
)

// countMiddleware adds 1 to the given queue counter, executes next
// (by calling ServeHTTP on it), then decrements the queue counter
func countMiddleware(
	lggr logr.Logger,
	routingTable routing.Table,
	q queue.Counter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpso := routingTable.Route(r)
		if httpso == nil {
			lggr.Error(nil, "not forwarding request")
			w.WriteHeader(400)
			if _, err := w.Write([]byte("Host not found, not forwarding request")); err != nil {
				lggr.Error(err, "could not write error message to client")
			}
			return
		}
		key := k8s.NamespacedNameFromObject(httpso).String()

		if err := q.Resize(key, +1); err != nil {
			lggr.Error(err, "Error incrementing queue", "key", key)
		}

		defer func() {
			if err := q.Resize(key, -1); err != nil {
				lggr.Error(err, "Error decrementing queue", "key", key)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
