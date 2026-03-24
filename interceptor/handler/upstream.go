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

	errNilUpstreamURL = errors.New("upstream URL is nil")
)

type Upstream struct {
	roundTripper   http.RoundTripper
	tracingCfg     config.Tracing
	shouldFallback bool
}

func NewUpstream(roundTripper http.RoundTripper, tracingCfg config.Tracing, shouldFallback bool) *Upstream {
	return &Upstream{
		roundTripper:   roundTripper,
		tracingCfg:     tracingCfg,
		shouldFallback: shouldFallback,
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

	url := util.UpstreamURLFromContext(ctx)
	if uh.shouldFallback {
		url = util.FallbackURLFromContext(ctx)
	}

	if url == nil {
		sh := NewStatic(http.StatusInternalServerError, errNilUpstreamURL)
		sh.ServeHTTP(w, r)

		return
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
		Transport:  uh.roundTripper,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			sh := NewStatic(http.StatusBadGateway, err)
			sh.ServeHTTP(w, r)
		},
	}

	proxy.ServeHTTP(w, r)
}
