package main

import (
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/queue"
)

// BuildAdminHandler creates the handler for the admin endpoint.
func BuildAdminHandler(logger logr.Logger, counter queue.Counter, probeHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/readyz", probeHandler)
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		// Best practice is to always return 200 as this indicates process health
		w.WriteHeader(http.StatusOK)
	})

	queue.AddCountsRoute(
		logger,
		mux,
		counter,
	)

	return mux
}
