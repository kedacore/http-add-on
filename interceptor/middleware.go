package main

import (
	"log"
	nethttp "net/http"

	"github.com/kedacore/http-add-on/pkg/queue"
)

// countMiddleware adds 1 to the given queue counter, executes next
// (by calling ServeHTTP on it), then decrements the queue counter
func countMiddleware(q queue.Counter, next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		host := r.Host
		if err := q.Resize(host, +1); err != nil {
			log.Printf("Error incrementing queue for %q (%s)", r.RequestURI, err)
		}
		defer func() {
			if err := q.Resize(host, -1); err != nil {
				log.Printf("Error decrementing queue for %q (%s)", r.RequestURI, err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
