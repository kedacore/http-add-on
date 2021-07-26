package routing

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/pkg/errors"
)

// StartUpdateLoop begins a loop that updates the given table t every
// updateInterval. The loop calls fetcher every updateInterval and,
// assuming the fetch succeeds, calls t.Replace(newTable) with the
// result (newTable is the return value of fetcher).
//
// if fetcher fails, this loop returns with a non-nil error
// immediately. if ctx.Done() ever receives, this loop also returns
// a non-nil error immediately, so if you want to stop it, pass a
// cancellable context and cancel it when you're ready
func StartUpdateLoop(
	ctx context.Context,
	lggr logr.Logger,
	httpCl *http.Client,
	fetchURL url.URL,
	updateInterval time.Duration,
	t *Table,
	q queue.Counter,
) error {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := GetTable(ctx, httpCl, lggr, fetchURL, t, q); err != nil {
				return errors.Wrap(err, "trying to fetch routing table")
			}
		case <-ctx.Done():
			return errors.New("context timeout")
		}
	}
}
