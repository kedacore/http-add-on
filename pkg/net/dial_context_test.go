package net

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestDialContextWithRetry(t *testing.T) {
	r := require.New(t)

	const (
		connTimeout = 10 * time.Millisecond
		keepAlive   = 10 * time.Millisecond
		network     = "tcp"
		addr        = "localhost:60001"
	)
	backoff := wait.Backoff{
		Duration: connTimeout,
		Factor:   2,
		Jitter:   0.5,
		Steps:    5,
	}

	ctx := context.Background()
	dialer := NewNetDialer(connTimeout, keepAlive)
	dRetry := DialContextWithRetry(dialer, backoff)
	minTotalWaitDur := MinTotalBackoffDuration(backoff)

	start := time.Now()
	_, err := dRetry(ctx, network, addr)
	r.Error(err, "error was not found")

	elapsed := time.Since(start)
	r.GreaterOrEqual(
		elapsed,
		minTotalWaitDur,
		"total elapsed (%s) was not >= than the minimum expected (%s)",
		elapsed,
		minTotalWaitDur,
	)
}
