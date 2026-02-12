package net

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"time"
)

// DialContextFunc matches the signature of net.Dialer.DialContext.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// retryInterval is kept short to minimize latency when the target becomes
// reachable. Not exposed as configuration — retryTimeout is the user-facing knob.
const retryInterval = 50 * time.Millisecond

// DialContextWithRetry retries failed dials at a fixed interval until
// retryTimeout expires or the parent context is cancelled.
func DialContextWithRetry(coreDialer *net.Dialer, retryTimeout time.Duration) DialContextFunc {
	// Default covers slow Service IP rule propagation to kube-proxy, Cilium, etc
	retryTimeout = cmp.Or(retryTimeout, 15*time.Second)

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, retryTimeout)
		defer cancel()

		// Track the last error to return it when the retry loop runs out of time
		var lastError error
		start := time.Now()

		for {
			conn, err := coreDialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
			lastError = err

			t := time.NewTimer(retryInterval)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, fmt.Errorf("retry dial %s after %.2fs: %w", addr, time.Since(start).Seconds(), lastError)
			case <-t.C:
			}
		}
	}
}

// NewNetDialer creates a new net.Dialer with the given connection timeout and
// keep alive duration.
func NewNetDialer(connectTimeout, keepAlive time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: keepAlive,
	}
}
