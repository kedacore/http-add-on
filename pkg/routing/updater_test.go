package routing

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
)

func TestStartUpdateLoop(t *testing.T) {
	r := require.New(t)
	lggr := logr.Discard()
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	const interval = 10 * time.Millisecond

	ch := make(chan struct{})
	go StartUpdateLoop(
		ctx,
		lggr,
		interval,
		func(ctx context.Context, cur time.Time) error {
			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
			}
			return nil
		},
	)
	go func() {
		time.Sleep(interval * 2)
		done()
		close(ch)
	}()
	numRecvs := 0
	for range ch {
		numRecvs++
	}
	r.Greater(
		numRecvs,
		0,
		"expected the update loop to execute at least once",
	)
}
