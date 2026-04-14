package net

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// DialContextFunc matches the signature of net.Dialer.DialContext.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

const (
	// retryInterval is kept short to minimize latency when the target becomes reachable.
	retryInterval = 50 * time.Millisecond

	// maxRetryDuration caps the total time spent retrying when the parent
	// context has no deadline as safe-guard for unreachable backends.
	maxRetryDuration = 1 * time.Minute
)

// DialContextWithRetry returns a DialContextFunc that retries failed dials at a
// fixed interval until the parent context is cancelled or its deadline expires.
// When the parent context has no deadline, retries are bounded by maxRetryDuration.
func DialContextWithRetry(connectTimeout time.Duration) DialContextFunc {
	dialer := net.Dialer{Timeout: connectTimeout}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Safety net: prevent infinite retries when no request deadline is set.
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, maxRetryDuration)
			defer cancel()
		}

		start := time.Now()

		conn, lastErr := dialer.DialContext(ctx, network, addr)
		if lastErr == nil {
			return conn, nil
		}

		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("retry dial %s after %.2fs: %w", addr, time.Since(start).Seconds(), errors.Join(ctx.Err(), lastErr))
			case <-ticker.C:
				conn, lastErr = dialer.DialContext(ctx, network, addr)
				if lastErr == nil {
					return conn, nil
				}
			}
		}
	}
}
