package middleware

import (
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

// StaticRouting intercepts requests that match an InterceptorRoute's static routes.
// Matched requests bypass the counting middleware entirely to avoid triggering scaling.
// When the backend is available the request is forwarded directly to upstream;
// when unavailable the configured static response is returned.
type StaticRouting struct {
	next       http.Handler
	upstream   http.Handler
	readyCache *k8s.ReadyEndpointsCache
	reader     client.Reader
}

// NewStaticRouting returns a middleware that evaluates static routes.
// next is the normal handler chain for requests not handled by this handler.
// upstream is the forwarding handler that bypasses counting and other scaling-related middleware.
func NewStaticRouting(next, upstream http.Handler, readyCache *k8s.ReadyEndpointsCache, reader client.Reader) *StaticRouting {
	return &StaticRouting{
		next:       next,
		upstream:   upstream,
		readyCache: readyCache,
		reader:     reader,
	}
}

var _ http.Handler = (*StaticRouting)(nil)

func (sr *StaticRouting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ir := util.InterceptorRouteFromContext(r.Context())

	matched := matchStaticRoute(r, ir.Spec.StaticRoutes)
	if matched == nil {
		sr.next.ServeHTTP(w, r)
		return
	}

	if matched.ResponseMode != httpv1beta1.StaticRouteResponseModeAlways {
		serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
		if sr.readyCache.HasReadyEndpoints(serviceKey) {
			sr.upstream.ServeHTTP(w, r)
			return
		}
	}

	serveStaticResponse(w, r, sr.reader, ir, &matched.Response, http.StatusOK)
}

// matchStaticRoute returns the first static route whose rules match the request, or nil.
// Multiple rules within a route use OR semantics; multiple routes use first-match-wins.
func matchStaticRoute(r *http.Request, routes []httpv1beta1.StaticRoute) *httpv1beta1.StaticRoute {
	for i := range routes {
		for _, rule := range routes[i].Rules {
			if routing.MatchRoutingRule(r, rule) {
				return &routes[i]
			}
		}
	}
	return nil
}
