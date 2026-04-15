package middleware

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/interceptor/handler"
	httpv1beta "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const defaultFallbackReadinessTimeout = 30 * time.Second

type EndpointResolverConfig struct {
	ReadinessTimeout      time.Duration
	EnableColdStartHeader bool
	Reader                client.Reader
	TLSEnabled            bool
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
// When the InterceptorRoute's ProxyMode is "Endpoint", it also resolves
// the upstream URL to a specific pod IP after readiness is confirmed.
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

	hasFallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil
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
	isColdStart, err := er.readyCache.WaitForReady(waitCtx, serviceKey)
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
		r = r.WithContext(ctx)
	}

	// When backend is ready and ProxyMode is Endpoint, override the upstream
	// URL with a direct pod IP picked from the endpoint cache.
	if err == nil && ir.Spec.ProxyMode == httpv1beta.ProxyModeEndpoint {
		endpointURL, resolveErr := er.resolveEndpointURL(ctx, ir)
		if resolveErr != nil {
			handler.NewStatic(http.StatusInternalServerError, resolveErr).ServeHTTP(w, r)
			return
		}
		ctx = util.ContextWithUpstreamURL(ctx, endpointURL)
		r = r.WithContext(ctx)
	}

	// isColdStart is only meaningful when the backend resolved without errors
	if err == nil && er.cfg.EnableColdStartHeader {
		w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
	}

	er.next.ServeHTTP(w, r)
}

// resolveEndpointURL picks a random ready pod from the endpoint cache and
// returns a URL pointing directly at that pod's IP and container port.
func (er *EndpointResolver) resolveEndpointURL(ctx context.Context, ir *httpv1beta.InterceptorRoute) (*url.URL, error) {
	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
	se := er.readyCache.GetEndpoints(serviceKey)
	if se == nil || len(se.Addresses) == 0 {
		return nil, fmt.Errorf("no ready endpoints for %s", serviceKey)
	}

	addr := se.Addresses[rand.IntN(len(se.Addresses))]

	port, err := er.resolveTargetPort(ctx, ir.Spec.Target, ir.Namespace, se)
	if err != nil {
		return nil, err
	}

	scheme := "http"
	if er.cfg.TLSEnabled {
		scheme = "https"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", addr, port),
	}, nil
}

// resolveTargetPort determines the container port for direct pod routing.
// For portName, it looks up the name directly in the cached EndpointSlice ports.
// For a numeric service port, it reads the Service to find the corresponding targetPort.
func (er *EndpointResolver) resolveTargetPort(ctx context.Context, target httpv1beta.TargetRef, namespace string, se *k8s.ServiceEndpoints) (int32, error) {
	if target.PortName != "" {
		port, ok := se.Ports[target.PortName]
		if !ok {
			return 0, fmt.Errorf("port name %q not found in endpoint slices", target.PortName)
		}
		return port, nil
	}

	if target.Port == 0 {
		return 0, fmt.Errorf(`must specify either "port" or "portName"`)
	}

	var svc corev1.Service
	if err := er.cfg.Reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: target.Service}, &svc); err != nil {
		return 0, fmt.Errorf("failed to get Service: %w", err)
	}

	for _, p := range svc.Spec.Ports {
		if p.Port != target.Port {
			continue
		}
		switch p.TargetPort.Type {
		case intstr.Int:
			if p.TargetPort.IntVal > 0 {
				return p.TargetPort.IntVal, nil
			}
			return p.Port, nil
		case intstr.String:
			port, ok := se.Ports[p.TargetPort.StrVal]
			if !ok {
				return 0, fmt.Errorf("target port name %q not found in endpoint slices", p.TargetPort.StrVal)
			}
			return port, nil
		}
	}
	return 0, fmt.Errorf("port %d not found in Service %s/%s", target.Port, namespace, target.Service)
}
