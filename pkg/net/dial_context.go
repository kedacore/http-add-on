package net

import (
	"context"
	"net"
	stdnet "net"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

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
func DialContextWithRetry(coreDialer *net.Dialer) DialContextFunc {
	return func(ctx context.Context, network, addr string) (stdnet.Conn, error) {
		backoff := wait.Backoff{
			Duration: coreDialer.Timeout,
			Factor:   2,
			Jitter:   0.5,
			Steps:    5,
		}
		var retConn stdnet.Conn
		try := 0
		err := wait.ExponentialBackoff(backoff, func() (bool, error) {
			try++
			conn, err := coreDialer.DialContext(ctx, network, addr)
			if err != nil {
				// if the dial failed, return false and an error so that the
				// backoff will continue, rather than just bailing out
				return false, nil
			}
			// if we succeeded in dialing, return true and no error
			retConn = conn
			return true, nil
		})
		if err != nil {
			// err will either be the error the internal function returned
			// or wait.ErrWaitTimeout
			return nil, err
		}
		return retConn, nil
	}

}

func NewNetDialer(connectTimeout, keepAlive time.Duration) *stdnet.Dialer {
	return &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: keepAlive,
	}
}
