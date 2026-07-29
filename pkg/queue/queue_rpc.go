package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

const countsPath = "/queue"

// maxCountsResponseSize bounds the response body accepted from an interceptor.
// The payload grows with the number of routes; 10 MiB holds well over 100k of
// them while still capping the memory a misbehaving interceptor can cost us.
const maxCountsResponseSize = 10 << 20

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

// GetCounts issues an RPC call to get the queue counts from the given
// interceptor. Note that interceptorURL should not end with a "/" and
// shouldn't include a path.
//
// The call is bounded by ctx and by httpCl.Timeout. Callers must ensure at
// least one of them is set: an interceptor that accepts the connection but
// never responds would otherwise block the caller indefinitely.
func GetCounts(
	ctx context.Context,
	httpCl *http.Client,
	interceptorURL url.URL,
) (Counts, error) {
	interceptorURL.Path = countsPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, interceptorURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("building the queue counts request for %s: %w", interceptorURL.String(), err)
	}

	resp, err := httpCl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting the queue counts from %s: %w", interceptorURL.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("requesting the queue counts from %s: unexpected status %s", interceptorURL.String(), resp.Status)
	}

	var counts Counts
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCountsResponseSize)).Decode(&counts); err != nil {
		return nil, fmt.Errorf("decoding response from the interceptor at %s: %w", interceptorURL.String(), err)
	}

	return counts, nil
}
