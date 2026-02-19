package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	ctrl "sigs.k8s.io/controller-runtime"
)

var proxyLog = ctrl.Log.WithName("proxy")

// ProxyServer is the main proxy handler.
//
// Architecture:
//
//  1. Accept request via Go's net/http server.
//  2. Route lookup — lock-free via atomic.Pointer (no mutex).
//  3. Queue increment — per-host atomic op (no global lock).
//  4. Endpoint check — single atomic load (fast path for warm backends).
//  5. Forward to backend via httputil.ReverseProxy with a custom Transport
//     that provides DNS caching and tuned connection pooling.
//  6. Record metrics + optional tracing / logging.
//
// Performance-critical improvements over the old interceptor:
//   - Lock-free routing table (was: radix tree behind RWMutex)
//   - Per-host atomic counters (was: single global RWMutex)
//   - Zero per-request goroutine spawning (was: 2 goroutines/request)
//   - Single flat handler (was: 4+ middleware wrappers per request)
//   - DNS caching in the Transport (was: per-connection resolution)
//
// Full HTTP protocol compliance via stdlib httputil.ReverseProxy:
//   - HTTP/1.1 and HTTP/2 support
//   - WebSocket / connection upgrade support
//   - Proper hop-by-hop header handling (RFC 7230)
//   - Trailer forwarding
//   - Expect: 100-continue
//   - Client disconnect propagation via context
//   - X-Forwarded-For chaining (appends, not overwrites)
type ProxyServer struct {
	config         *Config
	routingTable   *RoutingTable
	queue          *QueueCounter
	endpointsCache *EndpointsCache
	transport      *http.Transport
	metrics        *MetricsCollector
	tracer         trace.Tracer // nil when tracing is disabled
	tlsConfig      *tls.Config  // nil when TLS is disabled
}

// NewProxyServer creates a new proxy server.
func NewProxyServer(
	config *Config,
	rt *RoutingTable,
	q *QueueCounter,
	ec *EndpointsCache,
	transport *http.Transport,
	m *MetricsCollector,
	opts ...ProxyOption,
) *ProxyServer {
	ps := &ProxyServer{
		config:         config,
		routingTable:   rt,
		queue:          q,
		endpointsCache: ec,
		transport:      transport,
		metrics:        m,
	}
	for _, o := range opts {
		o(ps)
	}
	return ps
}

// ProxyOption configures optional ProxyServer features.
type ProxyOption func(*ProxyServer)

// WithTracer enables OpenTelemetry tracing on the proxy.
func WithTracer(t trace.Tracer) ProxyOption {
	return func(ps *ProxyServer) { ps.tracer = t }
}

// WithTLSConfig sets the TLS configuration for the TLS listener.
func WithTLSConfig(tc *tls.Config) ProxyOption {
	return func(ps *ProxyServer) { ps.tlsConfig = tc }
}

// ListenAndServe starts the plain HTTP proxy server.
func (ps *ProxyServer) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", ps.config.ProxyPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           ps,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
	proxyLog.Info("Proxy server listening", "addr", addr)
	return server.ListenAndServe()
}

// ListenAndServeTLS starts the TLS proxy server on the configured TLS port.
// Requires a non-nil tlsConfig (set via WithTLSConfig).
func (ps *ProxyServer) ListenAndServeTLS() error {
	if ps.tlsConfig == nil {
		return fmt.Errorf("TLS enabled but no TLS configuration provided")
	}
	addr := fmt.Sprintf(":%d", ps.config.TLSProxyPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           ps,
		TLSConfig:         ps.tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
	proxyLog.Info("TLS proxy server listening", "addr", addr)
	return server.ListenAndServeTLS("", "")
}

// ServeHTTP handles each proxy request.
func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	path := r.URL.Path
	method := r.Method
	start := time.Now()

	// ---- 0. OTel tracing (extract context, start span) ----
	var span trace.Span
	if ps.tracer != nil {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span = ps.tracer.Start(ctx, "proxy",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.target", path),
				attribute.String("http.host", host),
			),
		)
		defer span.End()
		r = r.WithContext(ctx)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))
	}

	// ---- 1. Route lookup (lock-free atomic.Pointer read) ----
	route := ps.routingTable.Route(host, path, r.Header)
	if route == nil {
		ps.recordAndLog(method, path, 404, host, start, span)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// ---- 2. Increment in-flight counter (atomic, no global lock) ----
	guard := ps.queue.Increase(route.QueueKey)
	defer guard.Release()

	// ---- 3. Wait for ready endpoints (cold-start support) ----
	waitTimeout := ps.config.ConditionWaitTimeout
	if route.ConditionWaitTimeout > 0 {
		waitTimeout = route.ConditionWaitTimeout
	} else if route.FailoverTimeout > 0 {
		waitTimeout = route.FailoverTimeout
	}

	isColdStart, waitErr := ps.endpointsCache.WaitForReady(route.ServiceKey, waitTimeout)
	authority := route.Authority
	if waitErr != nil {
		if route.HasFailover {
			isColdStart = true
			authority = route.FailoverAuthority
		} else {
			ps.recordAndLog(method, path, 502, host, start, span)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
	}

	// ---- 4. Cold-start connectivity probe ----
	if isColdStart {
		if err := ps.coldStartProbe(authority, host); err != nil {
			ps.recordAndLog(method, path, 502, host, start, span)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
	}

	// ---- 5. Apply per-route response timeout via context ----
	respTimeout := ps.config.ResponseHeaderTimeout
	if route.ResponseHeaderTimeout > 0 {
		respTimeout = route.ResponseHeaderTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), respTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	// ---- 6. Forward to backend (httputil.ReverseProxy) ----
	scheme := "http"
	if r.TLS != nil && ps.tlsConfig != nil {
		scheme = "https"
	}
	target := &url.URL{
		Scheme: scheme,
		Host:   authority,
	}

	statusCode := 502
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = r.Host
		},
		Transport: ps.transport,
		ModifyResponse: func(resp *http.Response) error {
			statusCode = resp.StatusCode
			if isColdStart && ps.config.EnableColdStartHeader {
				resp.Header.Set("X-KEDA-HTTP-Cold-Start", "true")
			}
			return nil
		},
		ErrorHandler: func(rw http.ResponseWriter, _ *http.Request, err error) {
			proxyLog.Info("proxy error",
				"error", err,
				"host", host,
				"method", method,
				"path", path,
			)
			statusCode = http.StatusBadGateway
			http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)

	// ---- 7. Record metrics ----
	ps.recordAndLog(method, path, statusCode, host, start, span)
}

// recordAndLog records the Prometheus metric, optionally annotates the OTel
// span, and optionally logs the request (when KEDA_HTTP_LOG_REQUESTS=true).
func (ps *ProxyServer) recordAndLog(method, path string, code int, host string, start time.Time, span trace.Span) {
	ps.metrics.RecordRequest(method, path, code, host)

	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", code))
	}

	if ps.config.LogRequests {
		proxyLog.Info("request",
			"method", method,
			"path", path,
			"host", host,
			"status", code,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// coldStartProbe attempts to connect to the backend with retries.
// This is necessary because endpoint readiness doesn't guarantee the
// backend is accepting TCP connections yet.
func (ps *ProxyServer) coldStartProbe(authority, host string) error {
	probeTimeout := 5 * time.Second
	deadline := time.Now().Add(probeTimeout)
	var attempt int

	for {
		conn, err := net.DialTimeout("tcp", authority, 1*time.Second)
		if err == nil {
			conn.Close()
			proxyLog.Info("cold-start: backend reachable",
				"host", host,
				"authority", authority,
				"attempt", attempt,
			)
			return nil
		}

		if time.Now().After(deadline) {
			proxyLog.Error(err, "cold-start: backend unreachable after probe timeout",
				"host", host,
				"authority", authority,
			)
			return err
		}

		attempt++
		delay := time.Duration(100<<min(attempt, 4)) * time.Millisecond
		proxyLog.V(1).Info("cold-start: probing backend connectivity",
			"host", host,
			"attempt", attempt,
			"delay", delay,
		)
		time.Sleep(delay)
	}
}
