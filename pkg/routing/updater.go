package routing

import (
	"context"
	"time"

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
	updateInterval time.Duration,
	t *Table,
	fetcher func(ctx context.Context) (*Table, error),
) error {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			newTable, err := fetcher(ctx)
			if err != nil {
				return errors.Wrap(err, "trying to fetch routing table")
			}
			t.Replace(newTable)
		case <-ctx.Done():
			return errors.New("context timeout")
		}
	}
}
