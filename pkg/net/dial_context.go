package net

import (
	"context"
	"fmt"
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
// Thanks to Knative for inspiring this code. See GitHub link below
// https://github.com/knative/serving/blob/20815258c92d0f26100031c71a91d0bef930a475/vendor/knative.dev/pkg/network/transports.go#L70
func DialContextWithRetry(coreDialer *net.Dialer, backoff wait.Backoff) DialContextFunc {
	numDialTries := backoff.Steps
	return func(ctx context.Context, network, addr string) (stdnet.Conn, error) {
		// note that we could test for backoff.Steps >= 0 here, but every call to backoff.Step()
		// (below) decrements the backoff.Steps value. If you accidentally call that function
		// more than once inside the loop, you will reduce the number of times the loop
		// executes. Using a standard counter makes this algorithm less likely to introduce
		// a bug
		var lastError error
		for i := 0; i < numDialTries; i++ {
			conn, err := coreDialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
			lastError = err
			sleepDur := backoff.Step()
			t := time.NewTimer(sleepDur)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, fmt.Errorf("context timed out: %s", ctx.Err())
			case <-t.C:
				t.Stop()
			}
		}
		return nil, lastError
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
