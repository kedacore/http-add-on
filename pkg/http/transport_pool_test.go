package http

import (
	"crypto/tls"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestTransportPool_ReusesSameTransport(t *testing.T) {
	pool := NewTransportPool(&http.Transport{})

	timeout := 5 * time.Second
	transport1 := pool.Get(timeout, "")
	transport2 := pool.Get(timeout, "")

	if transport1 != transport2 {
		t.Errorf("expected same transport instance for same timeout, got different instances")
	}
}

func TestTransportPool_DifferentTimeouts(t *testing.T) {
	pool := NewTransportPool(&http.Transport{})

	timeout1 := 5 * time.Second
	timeout2 := 10 * time.Second

	transport1 := pool.Get(timeout1, "")
	transport2 := pool.Get(timeout2, "")

	if transport1 == transport2 {
		t.Errorf("expected different transport instances for different timeouts, got same instance")
	}

	if transport1.ResponseHeaderTimeout != timeout1 {
		t.Errorf("expected transport1 ResponseHeaderTimeout to be %v, got %v", timeout1, transport1.ResponseHeaderTimeout)
	}
	if transport2.ResponseHeaderTimeout != timeout2 {
		t.Errorf("expected transport2 ResponseHeaderTimeout to be %v, got %v", timeout2, transport2.ResponseHeaderTimeout)
	}
}

func TestTransportPool_ConcurrentAccess(t *testing.T) {
	pool := NewTransportPool(&http.Transport{})

	timeout := 5 * time.Second
	numGoroutines := 100
	var wg sync.WaitGroup
	transports := make(chan *http.Transport, numGoroutines)

	for range numGoroutines {
		wg.Go(func() {
			transports <- pool.Get(timeout, "")
		})
	}

	wg.Wait()
	close(transports)

	var allTransports []*http.Transport
	for transport := range transports {
		allTransports = append(allTransports, transport)
	}

	firstTransport := allTransports[0]
	for i, transport := range allTransports {
		if transport != firstTransport {
			t.Errorf("goroutine %d got different transport instance", i)
		}
	}
}

func TestTransportPool_SameKeyReusesTransport(t *testing.T) {
	pool := NewTransportPool(&http.Transport{
		TLSClientConfig: &tls.Config{},
	})

	timeout := 5 * time.Second
	serverName := "my-svc.default"

	transport1 := pool.Get(timeout, serverName)
	transport2 := pool.Get(timeout, serverName)

	if transport1 != transport2 {
		t.Errorf("expected same transport instance for same (timeout, serverName), got different instances")
	}
}

func TestTransportPool_DifferentServerNamesDifferentTransports(t *testing.T) {
	pool := NewTransportPool(&http.Transport{
		TLSClientConfig: &tls.Config{},
	})

	timeout := 5 * time.Second

	transport1 := pool.Get(timeout, "svc-a.default")
	transport2 := pool.Get(timeout, "svc-b.default")

	if transport1 == transport2 {
		t.Errorf("expected different transport instances for different serverNames, got same instance")
	}
}

func TestTransportPool_TLS_ServerNameSetOnTransport(t *testing.T) {
	pool := NewTransportPool(&http.Transport{
		TLSClientConfig: &tls.Config{},
	})

	timeout := 5 * time.Second
	serverName := "my-svc.default"

	transport := pool.Get(timeout, serverName)

	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be non-nil")
	}
	if transport.TLSClientConfig.ServerName != serverName {
		t.Errorf("expected TLSClientConfig.ServerName to be %q, got %q", serverName, transport.TLSClientConfig.ServerName)
	}
}

// TestTransportPool_ServerNameWithNoTLS verifies that passing a non-empty serverName
// to a pool whose base transport has no TLSClientConfig (plain HTTP) is safe —
// the serverName is silently ignored and not applied to the cloned transport.
// This guards against Go's Clone() allocating a TLSClientConfig internally for
// HTTP/2 negotiation, which would cause the serverName to be stamped onto a
// plain-HTTP transport if the pool guarded on the clone's TLSClientConfig instead
// of the caller's original intent (captured via tlsEnabled at construction time).
func TestTransportPool_ServerNameWithNoTLS(t *testing.T) {
	pool := NewTransportPool(&http.Transport{
		// TLSClientConfig intentionally nil — plain HTTP base transport.
	})

	timeout := 5 * time.Second
	serverName := "my-svc.default"

	transport := pool.Get(timeout, serverName)

	if transport == nil {
		t.Fatal("expected a non-nil transport")
	}
	// ServerName must not be set even if Clone() allocated a TLSClientConfig internally.
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.ServerName == serverName {
		t.Errorf("serverName %q must not be applied to a non-TLS base transport", serverName)
	}
}
