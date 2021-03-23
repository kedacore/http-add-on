package main

import (
	"testing"
	"time"
)

// the proxy should successfully forward a request to a running server
func TestImmediatelySuccessfulProxy(t *testing.T) {
	t.Fatal("TODO")
}

// the proxy should wait for a timeout and fail if there is no origin to connect
// to
func TestWaitFailedConnection(t *testing.T) {
	t.Fatal("TODO")
}

// the proxy should connect to a server, and then time out if the server doesn't
// respond in time
func TestWaitHeaderTimeout(t *testing.T) {
	t.Fatal("TODO")
}

// ensureSignalAfter ensures that signalCh receives (or is closed) before
// timeout
func ensureSignalAfter(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-signalCh:
		return true
	}
}

// ensuireNoSignalAfter ensures that signalCh does not receive (and is not closed)
// within timeout
func ensureNoSignalAfter(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-signalCh:
		return false
	}
}
