package queue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

const countsPath = "/queue"

func AddCountsRoute(lggr logr.Logger, mux *http.ServeMux, q CountReader) {
	lggr = lggr.WithName("pkg.queue.AddCountsRoute")
	lggr.Info("adding queue counts route", "path", countsPath)
	mux.Handle(countsPath, newSizeHandler(lggr, q))
}

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func newSizeHandler(
	lggr logr.Logger,
	q CountReader,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur, err := q.Current()
		if err != nil {
			lggr.Error(err, "getting queue size")
			http.Error(w, "error getting queue size", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(cur); err != nil {
			lggr.Error(err, "encoding QueueCounts")
			http.Error(w, "error encoding queue counts", http.StatusInternalServerError)
			return
		}
	})
}

// GetQueueCounts issues an RPC call to get the queue counts
// from the given hostAndPort. Note that the hostAndPort should
// not end with a "/" and shouldn't include a path.
func GetCounts(
	httpCl *http.Client,
	interceptorURL url.URL,
) (Counts, error) {
	interceptorURL.Path = countsPath
	resp, err := httpCl.Get(interceptorURL.String())
	if err != nil {
		return nil, fmt.Errorf("requesting the queue counts from %s: %w", interceptorURL.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	var counts Counts
	if err := json.NewDecoder(resp.Body).Decode(&counts); err != nil {
		return nil, fmt.Errorf("decoding response from the interceptor at %s: %w", interceptorURL.String(), err)
	}

	return counts, nil
}
