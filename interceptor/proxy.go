package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
)

// ProxyHandlerConfig contains all dependencies for building the proxy handler chain.
type ProxyHandlerConfig struct {
	Logger       logr.Logger
	Queue        queue.Counter
	WaitFunc     forwardWaitFunc
	RoutingTable routing.Table
	Reader       client.Reader
	Timeouts     config.Timeouts
	Serving      config.Serving
	TLSConfig    *tls.Config
	Tracing      config.Tracing

	// dialAddressOverride redirects all dial attempts to this address (for testing).
	// If empty, dials to the original target address.
	dialAddressOverride string
}

// BuildProxyHandler constructs the proxy handler chain.
func BuildProxyHandler(cfg *ProxyHandlerConfig) http.Handler {
	dialer := kedanet.NewNetDialer(cfg.Timeouts.Connect, cfg.Timeouts.KeepAlive)
	dialFunc := kedanet.DialContextWithRetry(dialer, cfg.Timeouts.DialRetryTimeout)

	// Wrap dialer to redirect if override is set (for testing)
	if cfg.dialAddressOverride != "" {
		originalDialFunc := dialFunc
		dialFunc = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return originalDialFunc(ctx, network, cfg.dialAddressOverride)
		}
	}

	var forwardingTLSCfg *tls.Config
	if cfg.TLSConfig != nil {
		forwardingTLSCfg = &tls.Config{
			RootCAs:            cfg.TLSConfig.RootCAs,
			Certificates:       cfg.TLSConfig.Certificates,
			InsecureSkipVerify: cfg.TLSConfig.InsecureSkipVerify, //nolint:gosec // G402: user-configurable
		}
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialFunc,
		ForceAttemptHTTP2:     cfg.Timeouts.ForceHTTP2,
		MaxIdleConns:          cfg.Timeouts.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.Timeouts.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.Timeouts.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.Timeouts.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.Timeouts.ExpectContinueTimeout,
		TLSClientConfig:       forwardingTLSCfg,
	}

	// Build handler chain (innermost to outermost)
	var handler http.Handler

	handler = newForwardingHandler(
		cfg.Logger,
		transport,
		cfg.WaitFunc,
		newForwardingConfigFromTimeouts(cfg.Timeouts, cfg.Serving),
		cfg.Tracing,
	)

	handler = middleware.NewCountingMiddleware(cfg.Queue, handler)

	handler = middleware.NewRouting(
		cfg.RoutingTable,
		handler,
		cfg.Reader,
		cfg.TLSConfig != nil,
	)

	if cfg.Tracing.Enabled {
		handler = otelhttp.NewHandler(handler, "keda-http-interceptor")
	}

	if cfg.Serving.LogRequests {
		handler = middleware.NewLogging(cfg.Logger, handler)
	}

	handler = middleware.NewMetrics(handler)

	return handler
}
