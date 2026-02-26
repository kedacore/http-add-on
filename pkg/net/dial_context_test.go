package net

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
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

	dialer := NewNetDialer(100*time.Millisecond, 1*time.Second)
	dialRetry := DialContextWithRetry(dialer, 1*time.Second)

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

	dialer := NewNetDialer(50*time.Millisecond, 1*time.Second)
	dialRetry := DialContextWithRetry(dialer, 2*time.Second)

	conn, err := dialRetry(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected success after target becomes reachable: %v", err)
	}
	_ = conn.Close()
	<-ready
}

func TestDialContextWithRetry_StopsAtTimeout(t *testing.T) {
	dialer := NewNetDialer(1*time.Millisecond, 10*time.Millisecond)
	retryTimeout := 200 * time.Millisecond
	dialRetry := DialContextWithRetry(dialer, retryTimeout)

	start := time.Now()
	_, err := dialRetry(t.Context(), "tcp", getUnreachableAddr(t))
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
	dialer := NewNetDialer(10*time.Millisecond, 1*time.Second)
	dialRetry := DialContextWithRetry(dialer, 5*time.Second)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := dialRetry(ctx, "tcp", getUnreachableAddr(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
	// Should stop around the parent context timeout (100ms), not the retryTimeout (5s).
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed %v; should have stopped near parent context timeout", elapsed)
	}
}

func TestDialContextWithRetry_WrapsError(t *testing.T) {
	dialer := NewNetDialer(1*time.Millisecond, 10*time.Millisecond)
	dialRetry := DialContextWithRetry(dialer, 100*time.Millisecond)

	addr := getUnreachableAddr(t)
	_, err := dialRetry(t.Context(), "tcp", addr)
	if err == nil {
		t.Fatal("expected error")
	}
	// The underlying dial error should be unwrappable.
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Errorf("expected wrapped *net.OpError, got %T: %v", err, err)
	}
}
