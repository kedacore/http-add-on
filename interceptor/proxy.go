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
	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
)

// ProxyHandlerConfig contains all dependencies for building the proxy handler chain.
type ProxyHandlerConfig struct {
	Logger       logr.Logger
	Queue        queue.Counter
	ReadyCache   *k8s.ReadyEndpointsCache
	RoutingTable routing.Table
	Reader       client.Reader
	Timeouts     config.Timeouts
	Serving      config.Serving
	TLSConfig    *tls.Config
	Tracing      config.Tracing
	Instruments  *metrics.Instruments

	// dialAddressOverride redirects all dial attempts to this address (for testing).
	// If empty, dials to the original target address.
	dialAddressOverride string
}

// BuildProxyHandler constructs the proxy handler chain.
func BuildProxyHandler(cfg *ProxyHandlerConfig) http.Handler {
	dialFunc := kedanet.DialContextWithRetry(cfg.Timeouts.Connect)

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
			MinVersion:         cfg.TLSConfig.MinVersion,
			MaxVersion:         cfg.TLSConfig.MaxVersion,
			CipherSuites:       cfg.TLSConfig.CipherSuites,
			CurvePreferences:   cfg.TLSConfig.CurvePreferences,
		}
	}

	// Clone DefaultTransport to inherit Go's defaults for IdleConnTimeout, ...
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialFunc
	transport.ForceAttemptHTTP2 = cfg.Timeouts.ForceHTTP2
	transport.MaxIdleConns = cfg.Timeouts.MaxIdleConns
	transport.MaxIdleConnsPerHost = cfg.Timeouts.MaxIdleConnsPerHost
	transport.TLSClientConfig = forwardingTLSCfg

	// Build handler chain (innermost to outermost)
	var h http.Handler

	h = handler.NewUpstream(transport, cfg.Tracing, cfg.Timeouts.ResponseHeader)

	h = middleware.NewEndpointResolver(h, cfg.ReadyCache, middleware.EndpointResolverConfig{
		ReadinessTimeout:      cfg.Timeouts.Readiness,
		EnableColdStartHeader: cfg.Serving.EnableColdStartHeader,
	})

	h = middleware.NewCounting(h, cfg.Queue, cfg.Instruments)

	h = middleware.NewRouting(
		h,
		cfg.RoutingTable,
		cfg.Reader,
		cfg.TLSConfig != nil,
		cfg.Timeouts.Request,
	)

	h = middleware.NewMetrics(h, cfg.Instruments)

	if cfg.Serving.LogRequests {
		h = middleware.NewLogging(h, cfg.Logger)
	}

	if cfg.Tracing.Enabled {
		h = otelhttp.NewHandler(h, "keda-http-interceptor")
	}

	return h
}
