package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kedacore/http-add-on/interceptor/handler"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const defaultFallbackReadinessTimeout = 30 * time.Second

type EndpointResolverConfig struct {
	ReadinessTimeout      time.Duration
	EnableColdStartHeader bool
	DirectPodRouting      bool // route every request to a pod IP instead of the ClusterIP service
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
	isColdStart, podHost, err := er.readyCache.WaitForReady(waitCtx, serviceKey, util.UpstreamPortNameFromContext(ctx))
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

		// Direct-pod routing: rewrite upstream to the pod IP (SNI stays the
		// service hostname from context). Empty podHost leaves the URL as-is.
		if er.cfg.DirectPodRouting && podHost != "" {
			if upstreamURL := util.UpstreamURLFromContext(ctx); upstreamURL != nil {
				podURL := *upstreamURL
				podURL.Host = podHost
				ctx = util.ContextWithUpstreamURL(ctx, &podURL)
				r = r.WithContext(ctx)
			}
		}
	}

	er.next.ServeHTTP(w, r)
}
