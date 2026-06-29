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

	// Clone DefaultTransport to inherit Go's defaults for IdleConnTimeout, ...
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.DialContext = dialFunc
	baseTransport.MaxIdleConns = cfg.Timeouts.MaxIdleConns
	baseTransport.MaxIdleConnsPerHost = cfg.Timeouts.MaxIdleConnsPerHost

	// When TLS is enabled, use DialTLSContext to set ServerName per-dial from
	// context. This is required for direct-pod routing where the URL host is a
	// pod IP but SNI must remain the original service hostname.
	if cfg.TLSConfig != nil {
		baseTransport.DialTLSContext = newTLSDialer(cfg.TLSConfig, dialFunc)
	}

	// Build handler chain (innermost to outermost)
	upstream := handler.NewUpstream(baseTransport, cfg.Reader, cfg.Tracing, cfg.Timeouts.ResponseHeader)

	var h http.Handler = middleware.NewEndpointResolver(upstream, cfg.ReadyCache, middleware.EndpointResolverConfig{
		ReadinessTimeout:      cfg.Timeouts.Readiness,
		EnableColdStartHeader: cfg.Serving.EnableColdStartHeader,
		DirectPodRouting:      cfg.Serving.DirectPodRouting,
	})

	h = middleware.NewPlaceholder(h, cfg.ReadyCache, cfg.Reader)

	h = middleware.NewCounting(h, cfg.Queue, cfg.Instruments)

	h = middleware.NewStaticRouting(h, upstream, cfg.ReadyCache, cfg.Reader)

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
// context (the original service hostname captured by routing middleware). When
// direct-pod routing rewrites the URL to a pod IP, this ensures SNI stays the
// service hostname. If the context has no server name (e.g. non-routing paths),
// falls back to the hostname extracted from the dial address.
func newTLSDialer(baseCfg *tls.Config, dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		tlsCfg := baseCfg.Clone()
		// Advertise h2 in ALPN explicitly: a custom DialTLSContext disables
		// net/http's auto-h2, so without this HTTPS upstreams (incl. gRPC)
		// downgrade to HTTP/1.1 (golang/go#20645).
		tlsCfg.NextProtos = []string{"h2", "http/1.1"}
		tlsCfg.ServerName = util.UpstreamServerNameFromContext(ctx)
		if tlsCfg.ServerName == "" {
			host, _, err := net.SplitHostPort(addr)
			if err == nil {
				tlsCfg.ServerName = host
			}
		}
		return tls.Client(conn, tlsCfg), nil
	}
}
