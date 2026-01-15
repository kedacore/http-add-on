package http

import (
	nethttp "net/http"
	"sync"
	"time"
)

// TransportPool manages a pool of nethttp.Transport instances,
// reusing them based on response header timeout configuration.
//
// NOTE: Transports are never evicted, we expect a low cardinality of timeouts
type TransportPool struct {
	transports    sync.Map // map[time.Duration]*nethttp.Transport
	baseTransport *nethttp.Transport
}

// NewTransportPool creates a new transport pool with a base transport template
func NewTransportPool(baseTransport *nethttp.Transport) *TransportPool {
	return &TransportPool{
		baseTransport: baseTransport,
	}
}

// Get returns a cached or new transport for the given response header timeout.
func (tp *TransportPool) Get(responseHeaderTimeout time.Duration) *nethttp.Transport {
	// Fast path: check if transport already exists
	if val, ok := tp.transports.Load(responseHeaderTimeout); ok {
		return val.(*nethttp.Transport)
	}

	// Slow path: create new transport
	transport := tp.baseTransport.Clone()
	transport.ResponseHeaderTimeout = responseHeaderTimeout

	// Try to store it, but another goroutine might have created one concurrently
	actual, loaded := tp.transports.LoadOrStore(responseHeaderTimeout, transport)
	if loaded {
		return actual.(*nethttp.Transport)
	}
	return transport
}
