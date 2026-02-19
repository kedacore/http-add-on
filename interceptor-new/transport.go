package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// TransportConfig holds the parameters for building an http.Transport with
// DNS caching and tuned connection pool settings.
type TransportConfig struct {
	ConnectTimeout        time.Duration
	KeepAlive             time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DNSCacheTTL           time.Duration
	ForceHTTP2            bool
	TLSClientConfig       *tls.Config // nil = use default
}

// TransportStats exposes diagnostic counters for the admin debug endpoint.
type TransportStats struct {
	ConnEstablished atomic.Uint64
	DNSCacheHits    atomic.Uint64
	DNSCacheMisses  atomic.Uint64
}

// NewTransport creates an http.Transport with DNS caching and tuned
// connection pool settings for reverse-proxying to Kubernetes backends.
func NewTransport(cfg *TransportConfig, stats *TransportStats) *http.Transport {
	dnsCache := &dnsCache{
		entries: sync.Map{},
		ttl:     cfg.DNSCacheTTL,
		stats:   stats,
	}

	dialer := &net.Dialer{
		Timeout:   cfg.ConnectTimeout,
		KeepAlive: cfg.KeepAlive,
	}

	t := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			resolved, err := dnsCache.resolve(addr)
			if err != nil {
				return nil, err
			}
			conn, err := dialer.DialContext(ctx, network, resolved)
			if err != nil {
				return nil, err
			}
			if tc, ok := conn.(*net.TCPConn); ok {
				_ = tc.SetNoDelay(true)
			}
			stats.ConnEstablished.Add(1)
			return conn, nil
		},
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		ForceAttemptHTTP2:     cfg.ForceHTTP2,
		TLSClientConfig:       cfg.TLSClientConfig,
	}

	return t
}

// ---------------------------------------------------------------------------
// DNS cache
// ---------------------------------------------------------------------------

type dnsCache struct {
	entries sync.Map // string -> *cachedDNSEntry
	ttl     time.Duration
	stats   *TransportStats
}

type cachedDNSEntry struct {
	addr       string // resolved "ip:port"
	insertedAt time.Time
}

// resolve returns a cached address or performs DNS resolution.
// addr is in "host:port" format as passed by http.Transport.
func (c *dnsCache) resolve(addr string) (string, error) {
	// Fast path: cache hit
	if v, ok := c.entries.Load(addr); ok {
		entry := v.(*cachedDNSEntry)
		if time.Since(entry.insertedAt) < c.ttl {
			c.stats.DNSCacheHits.Add(1)
			return entry.addr, nil
		}
	}

	// Slow path: resolve
	c.stats.DNSCacheMisses.Add(1)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, nil // not host:port — return as-is
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return addr, nil // fallback to original
	}

	resolved := net.JoinHostPort(addrs[0], port)
	c.entries.Store(addr, &cachedDNSEntry{
		addr:       resolved,
		insertedAt: time.Now(),
	})

	return resolved, nil
}
