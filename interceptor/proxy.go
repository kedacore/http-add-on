package main

import (
	"context"
	"crypto/tls"
	"fmt"
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
	"github.com/kedacore/http-add-on/pkg/util"
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
			// Advertise h2 in ALPN explicitly: a custom TLSClientConfig +
			// DialTLSContext disables net/http's auto-h2, so without this HTTPS
			// upstreams (incl. gRPC) downgrade to HTTP/1.1 (golang/go#20645).
			NextProtos: []string{"h2", "http/1.1"},
		}
	}

	// Clone DefaultTransport to inherit Go's defaults for IdleConnTimeout, ...
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.DialContext = dialFunc
	baseTransport.MaxIdleConns = cfg.Timeouts.MaxIdleConns
	baseTransport.MaxIdleConnsPerHost = cfg.Timeouts.MaxIdleConnsPerHost
	baseTransport.TLSClientConfig = forwardingTLSCfg

	// Direct-pod routing rewrites the upstream URL to a pod IP, so SNI must come
	// from context (the original service hostname) rather than the dial address.
	if forwardingTLSCfg != nil {
		baseTransport.DialTLSContext = newTLSDialer(forwardingTLSCfg, dialFunc)
	}

	// Build handler chain (innermost to outermost)
	var h http.Handler

	h = handler.NewUpstream(baseTransport, cfg.Reader, cfg.Tracing, cfg.Timeouts.ResponseHeader)

	h = middleware.NewEndpointResolver(h, cfg.ReadyCache, middleware.EndpointResolverConfig{
		ReadinessTimeout:      cfg.Timeouts.Readiness,
		EnableColdStartHeader: cfg.Serving.EnableColdStartHeader,
		DirectPodRouting:      cfg.Serving.DirectPodRouting,
	})

	h = middleware.NewPlaceholder(h, cfg.ReadyCache, cfg.Reader)

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

// newTLSDialer dials TCP then wraps the conn in TLS, taking ServerName from
// context (not the dial address, which direct-pod routing rewrites to a pod IP).
// The SNI should never be empty here as Routing sets it before any https dial;
// the empty check is just a safety net so we fail closed instead of dialing
// without an explicit upstream identity.
func newTLSDialer(tlsCfg *tls.Config, dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		serverName := util.UpstreamServerNameFromContext(ctx)
		if serverName == "" {
			return nil, fmt.Errorf("upstream server name missing from context for TLS dial to %s (refusing to dial without explicit SNI)", addr)
		}
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		dialed := tlsCfg.Clone()
		dialed.ServerName = serverName
		return tls.Client(conn, dialed), nil
	}
}
