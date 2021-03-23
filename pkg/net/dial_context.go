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
// that calls coreDialer.DialContext multiple times with an exponential backoff. It's
// mainly used in the interceptor so that, once it sees that the target deployment has
// 1 or more replicas, it can forward the request. If the deployment's state changed
// in the time slice between detecting >=1 replicas and the network send, the connection
// will be retried a few times.
//
// Thanks to KNative for inspiring this code. See GitHub link below
// https://github.com/knative/serving/blob/1640d2755a7c61bdb65414ef552bfb511470ac70/vendor/knative.dev/pkg/network/transports.go#L64
func DialContextWithRetry(coreDialer *net.Dialer, stopAfter time.Duration) DialContextFunc {
	return func(ctx context.Context, network, addr string) (stdnet.Conn, error) {
		backoff := wait.Backoff{
			Duration: stopAfter,
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
				return true, err
			}
			retConn = conn
			return true, nil
		})
		if err != nil {
			return nil, err
		}
		return retConn, nil
	}

}

func NewNetDialer(connectTimeout, keepAlive time.Duration) *stdnet.Dialer {
	return &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: keepAlive, // 30 * time.Second,
	}
}
