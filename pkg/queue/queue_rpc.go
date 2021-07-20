package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	nethttp "net/http"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const CountsPath = "/queue"

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func NewSizeHandler(
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
// from the given completeAddr
func GetCounts(
	ctx context.Context,
	lggr logr.Logger,
	httpCl *nethttp.Client,
	completeAddr string,
) (*Counts, error) {
	resp, err := httpCl.Get(completeAddr)
	if err != nil {
		errMsg := fmt.Sprintf(
			"requesting the queue counts from %s",
			completeAddr,
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
				completeAddr,
			),
		)
	}

	return counts, nil
}
