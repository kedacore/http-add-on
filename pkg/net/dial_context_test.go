package net

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

// getUnreachableAddr returns an address that is guaranteed to be unreachable
// by allocating an available port and immediately closing it
func getUnreachableAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

func TestDialContextWithRetry(t *testing.T) {
	t.Run("retries with exponential backoff on connection failure", func(t *testing.T) {
		r := require.New(t)

		backoff := wait.Backoff{
			Duration: 10 * time.Millisecond,
			Factor:   2,
			Jitter:   0.1,
			Steps:    3,
		}

		dialer := NewNetDialer(5*time.Millisecond, 10*time.Millisecond)
		dialRetry := DialContextWithRetry(dialer, backoff)

		start := time.Now()
		_, err := dialRetry(context.Background(), "tcp", getUnreachableAddr(t))
		elapsed := time.Since(start)

		r.Error(err, "should fail when connecting to unreachable address")

		// Verify backoff was applied by checking we took at least the minimum backoff time
		minExpected := MinTotalBackoffDuration(backoff)
		r.GreaterOrEqual(elapsed, minExpected, "should take at least minimum backoff duration")
	})

	t.Run("succeeds immediately when connection available", func(t *testing.T) {
		r := require.New(t)

		backoff := wait.Backoff{
			Duration: 50 * time.Millisecond,
			Factor:   2,
			Jitter:   0.1,
			Steps:    5,
		}

		srv, srvURL, err := StartTestServer(NewTestHTTPHandlerWrapper(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		))
		r.NoError(err)
		defer srv.Close()

		dialer := NewNetDialer(100*time.Millisecond, 1*time.Second)
		dialRetry := DialContextWithRetry(dialer, backoff)

		start := time.Now()
		conn, err := dialRetry(context.Background(), "tcp", srvURL.Host)
		elapsed := time.Since(start)

		r.NoError(err)
		r.NotNil(conn)
		if conn != nil {
			conn.Close()
		}

		r.Less(elapsed, backoff.Duration)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		r := require.New(t)

		backoff := wait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   2,
			Jitter:   0.1,
			Steps:    5,
		}

		dialer := NewNetDialer(10*time.Millisecond, 1*time.Second)
		dialRetry := DialContextWithRetry(dialer, backoff)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := dialRetry(ctx, "tcp", getUnreachableAddr(t))
		elapsed := time.Since(start)

		r.Error(err)
		r.Contains(err.Error(), "context")
		r.Less(elapsed, MinTotalBackoffDuration(backoff))
	})
}
