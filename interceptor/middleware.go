package main

import (
	"fmt"
	"log"
	nethttp "net/http"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
)

func getHost(r *nethttp.Request) (string, error) {
	// check the host header first, then the request host
	// field (which may contain the actual URL if there is no
	// host header)
	if r.Header.Get("Host") != "" {
		return r.Header.Get("Host"), nil
	}
	if r.Host != "" {
		return r.Host, nil
	}
	return "", fmt.Errorf("host not found")
}

// countMiddleware adds 1 to the given queue counter, executes next
// (by calling ServeHTTP on it), then decrements the queue counter
func countMiddleware(
	lggr logr.Logger,
	q queue.Counter,
	next nethttp.Handler,
) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		host, err := getHost(r)
		if err != nil {
			lggr.Error(err, "not forwarding request")
			w.WriteHeader(400)
			w.Write([]byte("Host not found, not forwarding request"))
			return
		}
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
