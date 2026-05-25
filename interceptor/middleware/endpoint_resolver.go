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
	DirectPodOnColdStart  bool // route to pod IP directly during cold start
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
	isColdStart, podHost, err := er.readyCache.WaitForReady(waitCtx, serviceKey, ir.Spec.Target.PortName)
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
		// Update SNI to the fallback service hostname for TLS upstreams so the
		// transport uses the correct server name instead of the primary service's.
		// For non-TLS fallbacks the context may still hold the primary service's
		// server name, but the transport ignores it for plain HTTP — no update needed.
		if fallbackURL.Scheme == schemeHTTPS {
			ctx = util.ContextWithUpstreamServerName(ctx, fallbackURL.Hostname())
		}
		r = r.WithContext(ctx)
	} else {
		// isColdStart is only meaningful when the backend resolved without errors
		if er.cfg.EnableColdStartHeader {
			w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
		}

		// Cold-start direct-to-pod routing: rewrites upstream to a pod IP, reducing latency when kube-proxy rules are slow to propagate.
		// TLS SNI uses the original service hostname captured in context. Empty podHost leaves the upstream URL unchanged.
		if isColdStart && er.cfg.DirectPodOnColdStart && podHost != "" {
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
