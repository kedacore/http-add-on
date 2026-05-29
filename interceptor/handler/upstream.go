package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/kedacore/http-add-on/interceptor/config"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	bufferPool = newBufferPool()

	errNilUpstreamURL = errors.New("upstream URL is nil")
)

type Upstream struct {
	defaultTransportPool   *kedahttp.TransportPool
	http2OnlyTransportPool *kedahttp.TransportPool
	tracingCfg             config.Tracing
	responseHeaderTimeout  time.Duration
}

func NewUpstream(baseTransport *http.Transport, tracingCfg config.Tracing, responseHeaderTimeout time.Duration) *Upstream {
	defaultTransport, http2OnlyTransport := kedahttp.NewTransports(baseTransport)

	return &Upstream{
		defaultTransportPool:   kedahttp.NewTransportPool(defaultTransport),
		http2OnlyTransportPool: kedahttp.NewTransportPool(http2OnlyTransport),
		tracingCfg:             tracingCfg,
		responseHeaderTimeout:  responseHeaderTimeout,
	}
}

var _ http.Handler = (*Upstream)(nil)

func (uh *Upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "UpstreamHandler")
	ctx := r.Context()

	if uh.tracingCfg.Enabled {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(
			attribute.String("service", "keda-http-interceptor-proxy-upstream"),
			attribute.String("cold-start", w.Header().Get(kedahttp.HeaderColdStart)),
		)
	}

	url := util.UpstreamURLFromContext(ctx)

	if url == nil {
		sh := NewStatic(http.StatusInternalServerError, errNilUpstreamURL)
		sh.ServeHTTP(w, r)

		return
	}

	// Select transport with per-route or global response header timeout.
	responseHeaderTimeout := uh.responseHeaderTimeout
	if ir := util.InterceptorRouteFromContext(ctx); ir != nil {
		if ir.Spec.Timeouts.ResponseHeader != nil {
			responseHeaderTimeout = ir.Spec.Timeouts.ResponseHeader.Duration
		}
	}

	pool := uh.defaultTransportPool
	if r.ProtoMajor == 2 {
		pool = uh.http2OnlyTransportPool
	}
	transport := pool.Get(responseHeaderTimeout)

	var rt http.RoundTripper = transport
	if uh.tracingCfg.Enabled {
		rt = otelhttp.NewTransport(transport)
	}

	rc := http.NewResponseController(w)
	if err := rc.EnableFullDuplex(); err != nil {
		util.LoggerFromContext(ctx).Error(err, "could not enable full duplex on response writer, continuing")
	}

	// Close the request body to prevent a server panic when it tries to read
	// the next keep-alive request after EnableFullDuplex (golang/go#68560).
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(url)
			// Preserve original Host header (SetURL rewrites it by default).
			pr.Out.Host = pr.In.Host

			// Preserve and extend X-Forwarded-... headers from upstream proxies
			pr.Out.Header["X-Forwarded-For"] = pr.In.Header["X-Forwarded-For"]
			pr.SetXForwarded()
			if host := pr.In.Header.Get("X-Forwarded-Host"); host != "" {
				pr.Out.Header.Set("X-Forwarded-Host", host)
			}
			if proto := pr.In.Header.Get("X-Forwarded-Proto"); proto != "" {
				pr.Out.Header.Set("X-Forwarded-Proto", proto)
			}
		},
		BufferPool: bufferPool,
		Transport:  rt,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			code := http.StatusBadGateway
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				// Respond with 504 Gateway Timeout on timeouts to differentiate from general server errors
				code = http.StatusGatewayTimeout
			} else if errors.Is(err, context.DeadlineExceeded) {
				code = http.StatusGatewayTimeout
			}
			sh := NewStatic(code, err)
			sh.ServeHTTP(w, r)
		},
	}

	proxy.ServeHTTP(w, r)
}
