package main

import (
	context "context"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
)

// newFakeQueuePinger creates the machinery required for a fake
// queuePinger implementation, including a time.Ticker, then returns
// the ticker and the pinger. it is the caller's responsibility to
// call ticker.Stop() on the returned ticker.
func newFakeQueuePinger(
	ctx context.Context,
	lggr logr.Logger,
) (*time.Ticker, *queuePinger) {
	ticker := time.NewTicker(100 * time.Millisecond)
	pinger := newQueuePinger(
		ctx,
		lggr,
		func(
			ctx context.Context,
			namespace,
			serviceName string,
		) (*v1.Endpoints, error) {
			return &v1.Endpoints{}, nil
		},
		"testns",
		"testsvc",
		"8080",
		ticker,
	)
	return ticker, pinger
}
