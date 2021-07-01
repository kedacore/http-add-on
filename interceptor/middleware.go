package main

import (
	"log"
	nethttp "net/http"

	"github.com/kedacore/http-add-on/pkg/http"
)

// countMiddleware takes que MemoryQueue previously initiated and increments the
// size of it before sending the request to the original app, after the request
// is finished, it decrements the queue size
func countMiddleware(q http.QueueCounter, next nethttp.Handler) nethttp.Handler {
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
