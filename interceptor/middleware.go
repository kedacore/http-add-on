package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/queue"
)

func getHost(r *http.Request) (string, error) {
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

func getHostAndPath(r *http.Request) (string, error) {
	var host string
	if r.Header.Get("Host") != "" {
		host = r.Header.Get("Host")
	}
	if r.Host != "" {
		host = r.Host
	}
	// Avoid appending empty ("/") Path
	if host != "" {
		if r.URL.Path != "/" {
			return host + r.URL.Path, nil
		} else {
			return host, nil
		}
	}

	return "", fmt.Errorf("host not found")
}

// countMiddleware adds 1 to the given queue counter, executes next
// (by calling ServeHTTP on it), then decrements the queue counter
func countMiddleware(
	lggr logr.Logger,
	q queue.Counter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, err := getHost(r)
		if err != nil {
			lggr.Error(err, "not forwarding request")
			w.WriteHeader(400)
			if _, err := w.Write([]byte("Host not found, not forwarding request")); err != nil {
				lggr.Error(err, "could not write error message to client")
			}
			return
		}

		host_and_path_str := fmt.Sprintf("%s%s", host, r.URL.Path)

		if err := q.Resize(host_and_path_str, +1); err != nil {
			log.Printf("Error incrementing queue for %q (%s)", r.RequestURI, err)
		}
		defer func() {
			if err := q.Resize(host_and_path_str, -1); err != nil {
				log.Printf("Error decrementing queue for %q (%s)", r.RequestURI, err)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
