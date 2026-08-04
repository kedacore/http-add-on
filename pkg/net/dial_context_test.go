package net

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

// getUnreachableAddr returns an address that is guaranteed to be unreachable
// by allocating an available port and immediately closing it
func getUnreachableAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func TestDialContextWithRetry_SucceedsImmediately(t *testing.T) {
	srv, srvURL, err := StartTestServer(NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	))
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Close()

	dialRetry := DialContextWithRetry(100 * time.Millisecond)

	start := time.Now()
	conn, err := dialRetry(t.Context(), "tcp", srvURL.Host)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	_ = conn.Close()

	// Should connect well before the retry interval fires.
	if elapsed >= 50*time.Millisecond {
		t.Errorf("elapsed %v; should connect immediately", elapsed)
	}
}

func TestDialContextWithRetry_RetriesUntilReachable(t *testing.T) {
	addr := getUnreachableAddr(t)

	// Start a listener on the same address after a delay.
	ready := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		close(ready)
		conn, err := ln.Accept()
		if err != nil {
			_ = ln.Close()
			return
		}
		_ = conn.Close()
		_ = ln.Close()
	}()

	dialRetry := DialContextWithRetry(50 * time.Millisecond)

	conn, err := dialRetry(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected success after target becomes reachable: %v", err)
	}
	_ = conn.Close()
	<-ready
}

func TestDialContextWithRetry_StopsAtTimeout(t *testing.T) {
	// Retries are bounded by the parent context's deadline.
	dialFunc := DialContextWithRetry(1 * time.Millisecond)
	retryTimeout := 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), retryTimeout)
	defer cancel()

	start := time.Now()
	_, err := dialFunc(ctx, "tcp", getUnreachableAddr(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when connecting to unreachable address")
	}

	// Should stop around retryTimeout, with some slack for the last interval.
	maxExpected := retryTimeout + 50*time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("elapsed %v > max expected %v", elapsed, maxExpected)
	}
	// Should run for at least close to the timeout.
	minExpected := retryTimeout - 50*time.Millisecond
	if elapsed < minExpected {
		t.Errorf("elapsed %v < min expected %v", elapsed, minExpected)
	}
}

func TestDialContextWithRetry_RespectsParentContext(t *testing.T) {
	dialRetry := DialContextWithRetry(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := dialRetry(ctx, "tcp", getUnreachableAddr(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
	// Should stop around the parent context timeout (100ms).
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed %v; should have stopped near parent context timeout", elapsed)
	}
}

func TestDialContextWithRetry_WrapsError(t *testing.T) {
	dialRetry := DialContextWithRetry(1 * time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	addr := getUnreachableAddr(t)
	_, err := dialRetry(ctx, "tcp", addr)
	if err == nil {
		t.Fatal("expected error")
	}
	// The underlying dial error should be unwrappable.
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Errorf("expected wrapped *net.OpError, got %T: %v", err, err)
	}
	// The context error should be in the chain so callers can detect timeouts.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded in error chain, got: %v", err)
	}
}

func TestDialContextWithRetry_StopsAtMaxAttempts(t *testing.T) {
	dialRetry := DialContextWithRetry(10 * time.Millisecond)

	// The parent context has no deadline: without the attempt bound the dial
	// would retry for maxRetryDuration.
	ctx := ContextWithMaxDialAttempts(t.Context(), 3)

	start := time.Now()
	_, err := dialRetry(ctx, "tcp", getUnreachableAddr(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	var exhausted *DialAttemptsExhaustedError
	if !errors.As(err, &exhausted) {
		t.Fatalf("expected *DialAttemptsExhaustedError, got %T: %v", err, err)
	}
	if exhausted.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", exhausted.Attempts)
	}
	// The underlying dial error should stay unwrappable.
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Errorf("expected wrapped *net.OpError, got %T: %v", err, err)
	}
	// 3 attempts at a 50ms interval take ~100ms, far under the 1-minute cap.
	if elapsed > 10*time.Second {
		t.Errorf("elapsed %v; bounded dial should give up quickly", elapsed)
	}
}

func TestDialContextWithRetry_MaxAttemptsOneFailsImmediately(t *testing.T) {
	dialRetry := DialContextWithRetry(10 * time.Millisecond)
	ctx := ContextWithMaxDialAttempts(t.Context(), 1)

	start := time.Now()
	_, err := dialRetry(ctx, "tcp", getUnreachableAddr(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error")
	}
	var exhausted *DialAttemptsExhaustedError
	if !errors.As(err, &exhausted) {
		t.Fatalf("expected *DialAttemptsExhaustedError, got %T: %v", err, err)
	}
	if exhausted.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", exhausted.Attempts)
	}
	if elapsed >= 50*time.Millisecond {
		t.Errorf("elapsed %v; should fail before the first retry interval", elapsed)
	}
}

func TestDialContextWithRetry_BoundedDialSucceedsWithinBudget(t *testing.T) {
	// A bounded dial keeps retrying within its budget when the target comes up.
	addr := getUnreachableAddr(t)

	ready := make(chan struct{})
	go func() {
		time.Sleep(120 * time.Millisecond)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		close(ready)
		conn, err := ln.Accept()
		if err != nil {
			_ = ln.Close()
			return
		}
		_ = conn.Close()
		_ = ln.Close()
	}()

	dialRetry := DialContextWithRetry(50 * time.Millisecond)
	ctx := ContextWithMaxDialAttempts(t.Context(), 10)

	conn, err := dialRetry(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("expected success within attempt budget: %v", err)
	}
	_ = conn.Close()
	<-ready
}

// TestDialContextWithRetry_AttemptCountUntilCap verifies the legacy unbounded
// behavior precisely: with no request deadline, a dead endpoint is redialed
// every 50ms until the 1-minute safety cap — i.e. ~1200 attempts before giving
// up. synctest's fake clock runs the full minute instantly.
func TestDialContextWithRetry_AttemptCountUntilCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		countingDial := func(context.Context, string, string) (net.Conn, error) {
			attempts++
			return nil, errors.New("connection refused")
		}
		dialRetry := dialContextWithRetry(countingDial)

		// No deadline on the parent context: retries are bounded only by
		// maxRetryDuration (1 minute).
		_, err := dialRetry(context.Background(), "tcp", "10.0.0.1:8080")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded in error chain, got: %v", err)
		}

		// 1 initial attempt + one per 50ms tick over 60s = ~1200. The final
		// tick races the deadline timer, so allow an off-by-one.
		if attempts < 1199 || attempts > 1201 {
			t.Errorf("attempts = %d, want ~1200 (20/s for 60s)", attempts)
		}
	})
}

// TestDialContextWithRetry_BoundedAttemptCount verifies the per-endpoint bound
// used for endpoint failover: exactly maxAttempts dials, then
// DialAttemptsExhaustedError.
func TestDialContextWithRetry_BoundedAttemptCount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		countingDial := func(context.Context, string, string) (net.Conn, error) {
			attempts++
			return nil, errors.New("connection refused")
		}
		dialRetry := dialContextWithRetry(countingDial)

		ctx := ContextWithMaxDialAttempts(context.Background(), 3)
		_, err := dialRetry(ctx, "tcp", "10.0.0.1:8080")

		var exhausted *DialAttemptsExhaustedError
		if !errors.As(err, &exhausted) {
			t.Fatalf("expected *DialAttemptsExhaustedError, got %T: %v", err, err)
		}
		if attempts != 3 {
			t.Errorf("attempts = %d, want exactly 3", attempts)
		}
	})
}
