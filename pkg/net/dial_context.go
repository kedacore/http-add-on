package net

import (
	"context"
	"net"
	stdnet "net"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// DialContextFunc is a function that matches the (net).Dialer.DialContext functions's
// signature
type DialContextFunc func(ctx context.Context, network, addr string) (stdnet.Conn, error)

// DialContextWithRetry creates a new DialContextFunc --
// which has the same signature as the net.Conn.DialContext function --
// that calls coreDialer.DialContext multiple times with an exponential backoff.
// The timeout of the first dial will be coreDialer.Timeout, and it will
// multiply by a factor of two from there.
//
// This function is mainly used in the interceptor so that, once it sees that the target deployment has
// 1 or more replicas, it can forward the request. If the deployment's state changed
// in the time slice between detecting >=1 replicas and the network send, the connection
// will be retried a few times.
//
// Thanks to KNative for inspiring this code. See GitHub link below
// https://github.com/knative/serving/blob/1640d2755a7c61bdb65414ef552bfb511470ac70/vendor/knative.dev/pkg/network/transports.go#L64
func DialContextWithRetry(coreDialer *net.Dialer, backoff wait.Backoff) DialContextFunc {
	return func(ctx context.Context, network, addr string) (stdnet.Conn, error) {
		for backoff.Steps > 0 {
			conn, err := coreDialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
			// NOTE: make sure to call backoff.Step() only once per loop iteration.
			// it decrements backoff.Steps every call. backoff.Steps is the number
			// of steps left in the backoff, and it's used in the loop iteration.
			sleepDur := backoff.Step()
			t := time.NewTimer(sleepDur)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, wait.ErrWaitTimeout
			case <-t.C:
				t.Stop()
			}
		}
		return nil, wait.ErrWaitTimeout
	}
}

// NewNetDialer creates a new (net).Dialer with the given connection timeout and
// keep alive duration.
func NewNetDialer(connectTimeout, keepAlive time.Duration) *stdnet.Dialer {
	return &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: keepAlive,
	}
}
