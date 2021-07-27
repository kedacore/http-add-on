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

// StartUpdateLoop begins a loop that calls fn every updateInterval.
// if fn returns an error, returns that error immediately and stops
// calling it. similarly, if ctx.Done() receives, this function
// stops calling fn and returns an error that wraps ctx.Err()
// immediately
func StartUpdateLoop(
	ctx context.Context,
	lggr logr.Logger,
	updateInterval time.Duration,
	fn func(context.Context, time.Time) error,
	// t *Table,
	// q queue.Counter,
) error {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := fn(ctx, time.Now()); err != nil {
				return errors.Wrap(err, "trying to fetch routing table")
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context timeout")
		}
	}
}

func NewGetTableUpdateLoopFunc(
	lggr logr.Logger,
	httpCl *http.Client,
	fetchURL url.URL,
	table *Table,
	q queue.Counter,
) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		return GetTable(ctx, httpCl, lggr, fetchURL, table, q)
	}
}
