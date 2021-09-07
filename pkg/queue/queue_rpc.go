package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	nethttp "net/http"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const countsPath = "/queue"

func AddCountsRoute(lggr logr.Logger, mux *nethttp.ServeMux, q CountReader) {
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
) nethttp.Handler {
	return http.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {

		cur, err := q.Current()
		if err != nil {
			lggr.Error(err, "getting queue size")
			w.WriteHeader(500)
			w.Write([]byte(
				"error getting queue size",
			))
			return
		}
		if err := json.NewEncoder(w).Encode(cur); err != nil {
			lggr.Error(err, "encoding QueueCounts")
			w.WriteHeader(500)
			w.Write([]byte(
				"error encoding queue counts",
			))
			return
		}
	})
}

// GetQueueCounts issues an RPC call to get the queue counts
// from the given hostAndPort. Note that the hostAndPort should
// not end with a "/" and shouldn't include a path.
func GetCounts(
	ctx context.Context,
	lggr logr.Logger,
	httpCl *nethttp.Client,
	interceptorURL url.URL,
) (*Counts, error) {
	interceptorURL.Path = countsPath
	resp, err := httpCl.Get(interceptorURL.String())
	if err != nil {
		errMsg := fmt.Sprintf(
			"requesting the queue counts from %s",
			interceptorURL.String(),
		)
		return nil, errors.Wrap(err, errMsg)
	}
	defer resp.Body.Close()
	counts := NewCounts()
	if err := json.NewDecoder(resp.Body).Decode(counts); err != nil {
		return nil, errors.Wrap(
			err,
			fmt.Sprintf(
				"decoding response from the interceptor at %s",
				interceptorURL.String(),
			),
		)
	}

	return counts, nil
}
