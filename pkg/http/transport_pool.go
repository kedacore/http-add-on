package http

import (
	nethttp "net/http"
	"sync"
	"time"
)

// transportKey is the composite key for the transport pool.
type transportKey struct {
	responseHeaderTimeout time.Duration
	serverName            string
}

// TransportPool manages a pool of nethttp.Transport instances,
// reusing them based on response header timeout and TLS server name.
// serverName is only applied when the base transport was explicitly configured
// with TLS; for plain-HTTP upstreams the key degenerates to timeout-only.
//
// NOTE: Transports are never evicted; we expect a low cardinality of (timeout, serverName) combinations.
type TransportPool struct {
	transports    sync.Map // map[transportKey]*nethttp.Transport
	baseTransport *nethttp.Transport
	// tlsEnabled is captured at construction time, before any Clone() call.
	// Go's Clone() triggers onceSetNextProtoDefaults which allocates a TLSClientConfig
	// on the base transport for HTTP/2 — even when the caller never set one. Checking
	// the clone's TLSClientConfig is therefore unreliable as a proxy for caller intent.
	tlsEnabled bool
}

// NewTransportPool creates a new transport pool with a base transport template.
func NewTransportPool(baseTransport *nethttp.Transport) *TransportPool {
	return &TransportPool{
		baseTransport: baseTransport,
		tlsEnabled:    baseTransport.TLSClientConfig != nil,
	}
}

// Get returns a cached or new transport for the given response header timeout and server name.
// serverName is applied as TLSClientConfig.ServerName only when the base transport was
// explicitly configured with TLS. Pass "" for non-TLS upstreams.
func (tp *TransportPool) Get(responseHeaderTimeout time.Duration, serverName string) *nethttp.Transport {
	key := transportKey{responseHeaderTimeout, serverName}

	// Fast path: check if transport already exists
	if val, ok := tp.transports.Load(key); ok {
		return val.(*nethttp.Transport)
	}

	// Slow path: create new transport
	transport := tp.baseTransport.Clone()
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	if serverName != "" && tp.tlsEnabled {
		transport.TLSClientConfig.ServerName = serverName
	}

	// Try to store it, but another goroutine might have created one concurrently
	actual, loaded := tp.transports.LoadOrStore(key, transport)
	if loaded {
		return actual.(*nethttp.Transport)
	}
	return transport
}
