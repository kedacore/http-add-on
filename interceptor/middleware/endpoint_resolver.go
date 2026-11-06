package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const defaultFallbackReadinessTimeout = 30 * time.Second

type EndpointResolverConfig struct {
	ReadinessTimeout      time.Duration
	EnableColdStartHeader bool
	DirectPodRouting      bool // route every request to a pod IP instead of the ClusterIP service
	Instruments           *metrics.Instruments
}

type EndpointResolver struct {
	next       http.Handler
	readyCache *k8s.ReadyEndpointsCache
	cfg        EndpointResolverConfig
}

// NewEndpointResolver returns a middleware that resolves a ready backend
// endpoint for each request. It waits for at least one endpoint to become
// ready (handling cold starts) and optionally falls back to an alternate
// upstream when the backend does not become ready in time.
func NewEndpointResolver(next http.Handler, readyCache *k8s.ReadyEndpointsCache, cfg EndpointResolverConfig) *EndpointResolver {
	return &EndpointResolver{
		next:       next,
		readyCache: readyCache,
		cfg:        cfg,
	}
}

var _ http.Handler = (*EndpointResolver)(nil)

func (er *EndpointResolver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ir := util.InterceptorRouteFromContext(ctx)

	readinessTimeout := er.cfg.ReadinessTimeout
	// Per-route override from InterceptorRoute spec
	if ir.Spec.Timeouts.Readiness != nil {
		readinessTimeout = ir.Spec.Timeouts.Readiness.Duration
	}

	hasFallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil && ir.Spec.ColdStart.Fallback.Service != nil
	// Bound the readiness wait or otherwise there is no time for the fallback
	if hasFallback && readinessTimeout == 0 {
		readinessTimeout = defaultFallbackReadinessTimeout
	}

	waitCtx := ctx
	if readinessTimeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, readinessTimeout)
		defer cancel()
	}

	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
	waitStart := time.Now()
	isColdStart, err := er.readyCache.WaitForReady(waitCtx, serviceKey)
	if info := routeInfoFromContext(ctx); info != nil {
		// An error means the request waited for readiness but the backend did
		// not become ready before the wait ended.
		info.IsColdStart = isColdStart || err != nil
	}

	if er.cfg.Instruments != nil && (isColdStart || err != nil) {
		outcome := metrics.ColdStartOutcomeReady
		switch {
		case errors.Is(err, context.Canceled):
			outcome = metrics.ColdStartOutcomeCancelled
		case err != nil:
			outcome = metrics.ColdStartOutcomeTimeout
		}
		er.cfg.Instruments.RecordColdStartDuration(ir.Name, ir.Namespace, outcome, time.Since(waitStart))
	}
	if err != nil {
		// No fallback, return an error
		if !hasFallback {
			code := http.StatusBadGateway
			// Context expired or aborted — no time remaining to reach the backend.
			if waitCtx.Err() != nil {
				code = http.StatusGatewayTimeout
			}
			handler.
				NewStatic(code, fmt.Errorf("backend not ready: %w", err)).
				ServeHTTP(w, r)
			return
		}

		// Has fallback but parent context expired, error early
		if ctx.Err() != nil {
			handler.
				NewStatic(http.StatusGatewayTimeout, fmt.Errorf("backend not ready and no time remaining for fallback: %w", err)).
				ServeHTTP(w, r)
			return
		}

		// Fall back to alternate upstream.
		fallbackURL := util.FallbackURLFromContext(ctx)
		ctx = util.ContextWithUpstreamURL(ctx, fallbackURL)
		// Swapping to the fallback URL: refresh the SNI so TLS doesn't present
		// the primary service's hostname (HTTP ignores ServerName).
		if fallbackURL.Scheme == "https" {
			ctx = util.ContextWithUpstreamServerName(ctx, fallbackURL.Hostname())
		}
		r = r.WithContext(ctx)
	} else {
		if er.cfg.EnableColdStartHeader {
			w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
		}

		if er.cfg.DirectPodRouting {
			r = er.routeToPod(r, ir, serviceKey)
		}
	}

	er.next.ServeHTTP(w, r)
}

// routeToPod rewrites the upstream URL to a ready pod of the service (SNI
// stays the service hostname from context). Leaves the request unchanged when
// no pod is available for the upstream port.
//
// With session persistence, the pod referenced by the session cookie is
// preferred. When the session gets a new pod, the cookie to issue is stored in
// the context for the upstream handler to set on the response.
func (er *EndpointResolver) routeToPod(r *http.Request, ir *httpv1beta1.InterceptorRoute, serviceKey string) *http.Request {
	ctx := r.Context()
	upstreamURL := util.UpstreamURLFromContext(ctx)
	if upstreamURL == nil {
		return r
	}

	session := newSessionCookie(ir)
	preferredID := ""
	if session != nil {
		preferredID = session.podID(r)
	}

	ep, ok := er.readyCache.PickEndpoint(serviceKey, util.UpstreamPortNameFromContext(ctx), preferredID)
	if !ok {
		return r
	}

	podURL := *upstreamURL
	podURL.Host = ep.Host
	ctx = util.ContextWithUpstreamURL(ctx, &podURL)
	if session != nil && ep.ID != preferredID {
		ctx = util.ContextWithSessionCookie(ctx, session.issue(r, ep.ID))
	}
	return r.WithContext(ctx)
}
