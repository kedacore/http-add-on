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
	errNilStream = errors.New("context stream is nil")
)

type Upstream struct {
	roundTripper   http.RoundTripper
	tracingCfg     *config.Tracing
	shouldFailover bool
}

func NewUpstream(roundTripper http.RoundTripper, tracingCfg *config.Tracing, shouldFailover bool) *Upstream {
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

	proxy := httputil.NewSingleHostReverseProxy(stream)
	superDirector := proxy.Director
	proxy.Transport = uh.roundTripper
	proxy.Director = func(req *http.Request) {
		superDirector(req)
		req.URL = stream
		req.URL.Path = r.URL.Path
		req.URL.RawPath = r.URL.RawPath
		req.URL.RawQuery = r.URL.RawQuery
		// delete the incoming X-Forwarded-For header so the proxy
		// puts its own in. This is also important to prevent IP spoofing
		req.Header.Del("X-Forwarded-For ")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		sh := NewStatic(http.StatusBadGateway, err)
		sh.ServeHTTP(w, r)
	}

	proxy.ServeHTTP(w, r)
}
