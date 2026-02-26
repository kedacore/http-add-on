package handler

import (
	"errors"
	"net/http"
	"net/http/httputil"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	bufferPool = newBufferPool()

	errNilStream = errors.New("context stream is nil")
)

type Upstream struct {
	roundTripper   http.RoundTripper
	tracingCfg     config.Tracing
	shouldFailover bool
}

func NewUpstream(roundTripper http.RoundTripper, tracingCfg config.Tracing, shouldFailover bool) *Upstream {
	return &Upstream{
		roundTripper:   roundTripper,
		tracingCfg:     tracingCfg,
		shouldFailover: shouldFailover,
	}
}

var _ http.Handler = (*Upstream)(nil)

func (uh *Upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "UpstreamHandler")
	ctx := r.Context()

	if uh.tracingCfg.Enabled {
		p := otel.GetTextMapPropagator()
		ctx = p.Extract(ctx, propagation.HeaderCarrier(r.Header))

		p.Inject(ctx, propagation.HeaderCarrier(w.Header()))

		span := trace.SpanFromContext(ctx)
		defer span.End()

		serviceValAttr := attribute.String("service", "keda-http-interceptor-proxy-upstream")
		coldStartValAttr := attribute.String("cold-start", w.Header().Get("X-KEDA-HTTP-Cold-Start"))

		span.SetAttributes(serviceValAttr, coldStartValAttr)
	}

	stream := util.StreamFromContext(ctx)
	if uh.shouldFailover {
		stream = util.FailoverStreamFromContext(ctx)
	}

	if stream == nil {
		sh := NewStatic(http.StatusInternalServerError, errNilStream)
		sh.ServeHTTP(w, r)

		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(stream)
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
		Transport:  uh.roundTripper,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			sh := NewStatic(http.StatusBadGateway, err)
			sh.ServeHTTP(w, r)
		},
	}

	//nolint:gosec // G704: reverse proxy forwards requests by design
	proxy.ServeHTTP(w, r)
}
