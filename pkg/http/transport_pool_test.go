package http

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestTransportPool_ReusesSameTransport(t *testing.T) {
	pool := NewTransportPool(&http.Transport{})

	timeout := 5 * time.Second
	transport1 := pool.Get(timeout)
	transport2 := pool.Get(timeout)

	if transport1 != transport2 {
		t.Errorf("expected same transport instance for same timeout, got different instances")
	}
}

func TestTransportPool_DifferentTimeouts(t *testing.T) {
	pool := NewTransportPool(&http.Transport{})

	timeout1 := 5 * time.Second
	timeout2 := 10 * time.Second

	transport1 := pool.Get(timeout1)
	transport2 := pool.Get(timeout2)

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
		wg.Add(1)
		go func() {
			defer wg.Done()
			transports <- pool.Get(timeout)
		}()
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
