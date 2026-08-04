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

// DialAttemptsExhaustedError is returned when a dial bounded by
// ContextWithMaxDialAttempts spends its per-endpoint attempt budget before the
// context deadline. It signals that failing over to another endpoint may
// succeed where redialing this one did not.
type DialAttemptsExhaustedError struct {
	Addr     string
	Attempts int
	Err      error
}

func (e *DialAttemptsExhaustedError) Error() string {
	return fmt.Sprintf("dial %s: %d attempt(s) exhausted: %v", e.Addr, e.Attempts, e.Err)
}

func (e *DialAttemptsExhaustedError) Unwrap() error { return e.Err }

type dialContextKey int

const ckMaxDialAttempts dialContextKey = iota

// ContextWithMaxDialAttempts bounds how many attempts DialContextWithRetry
// makes for a single endpoint before giving up with a
// *DialAttemptsExhaustedError. A value <= 0 (or no value at all) keeps the
// default behavior: retry until the context is cancelled or its deadline
// expires.
func ContextWithMaxDialAttempts(ctx context.Context, maxAttempts int) context.Context {
	return context.WithValue(ctx, ckMaxDialAttempts, maxAttempts)
}

func maxDialAttemptsFromContext(ctx context.Context) int {
	cv, _ := ctx.Value(ckMaxDialAttempts).(int)
	return cv
}

// DialContextWithRetry returns a DialContextFunc that retries failed dials at a
// fixed interval until the parent context is cancelled or its deadline expires.
// When the parent context has no deadline, retries are bounded by maxRetryDuration.
// When the context carries a max-attempts bound (see ContextWithMaxDialAttempts),
// the dial gives up after that many failed attempts instead.
func DialContextWithRetry(connectTimeout time.Duration) DialContextFunc {
	dialer := net.Dialer{Timeout: connectTimeout}
	return dialContextWithRetry(dialer.DialContext)
}

// dialContextWithRetry is DialContextWithRetry with the underlying dial
// function injected, so tests can count attempts and control failures.
func dialContextWithRetry(dial DialContextFunc) DialContextFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Safety net: prevent infinite retries when no request deadline is set.
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, maxRetryDuration)
			defer cancel()
		}

		maxAttempts := maxDialAttemptsFromContext(ctx)
		start := time.Now()

		conn, lastErr := dial(ctx, network, addr)
		if lastErr == nil {
			return conn, nil
		}

		attempts := 1
		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()

		for {
			if maxAttempts > 0 && attempts >= maxAttempts {
				return nil, &DialAttemptsExhaustedError{Addr: addr, Attempts: attempts, Err: lastErr}
			}
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("retry dial %s after %.2fs: %w", addr, time.Since(start).Seconds(), errors.Join(ctx.Err(), lastErr))
			case <-ticker.C:
				conn, lastErr = dial(ctx, network, addr)
				if lastErr == nil {
					return conn, nil
				}
				attempts++
			}
		}
	}
}
