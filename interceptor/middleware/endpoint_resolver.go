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

type EndpointResolverConfig struct {
	WaitTimeout           time.Duration
	EnableColdStartHeader bool
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

	waitTimeout := er.cfg.WaitTimeout

	// Annotation-based timeout override
	// TODO(v1): remove timeout compatibility fallback for HTTPSO before v1 release
	if v, ok := ir.Annotations[k8s.AnnotationConditionWaitTimeout]; ok {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			waitTimeout = d
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
	isColdStart, err := er.readyCache.WaitForReady(waitCtx, serviceKey)

	hasFallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil
	if err != nil && !hasFallback {
		handler.
			NewStatic(http.StatusBadGateway, fmt.Errorf("backend not ready: %w", err)).
			ServeHTTP(w, r)
		return
	}

	if er.cfg.EnableColdStartHeader {
		w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
	}

	if hasFallback && err != nil {
		fallbackURL := util.FallbackURLFromContext(ctx)
		ctx = util.ContextWithUpstreamURL(ctx, fallbackURL)
		r = r.WithContext(ctx)
	}

	er.next.ServeHTTP(w, r)
}
