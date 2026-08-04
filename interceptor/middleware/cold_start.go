package middleware

import (
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

// ColdStartConfig configures the ColdStart middleware.
type ColdStartConfig struct {
	// MaxPendingRequests is the default limit on requests held per route
	// while its backend has no ready endpoints. Routes override it via
	// coldStart.maxPendingRequests. Zero or negative means unlimited.
	MaxPendingRequests int
}

// ColdStart decides what happens to a request while its target has no ready
// endpoints (e.g. during scale-from-zero):
//   - serves the route's placeholder response immediately, or
//   - passes the request through to be held until an endpoint becomes ready,
//     bounded per route by coldStart.maxPendingRequests, and on overflow
//     either rejects it with 503 or serves the placeholder response.
//
// It sits after the Counting middleware so held and placeholder-served
// requests are counted — that count is what drives scale-from-zero.
type ColdStart struct {
	next         http.Handler
	readyCache   *k8s.ReadyEndpointsCache
	reader       client.Reader
	queueCounter queue.Counter
	instruments  *metrics.Instruments
	cfg          ColdStartConfig
}

// NewColdStart returns a middleware handling requests while the target has
// no ready endpoints. The reader resolves response bodies stored in
// ConfigMaps; the queue counter provides the per-route pending request count
// maintained by the Counting middleware.
func NewColdStart(next http.Handler, readyCache *k8s.ReadyEndpointsCache, reader client.Reader, queueCounter queue.Counter, instruments *metrics.Instruments, cfg ColdStartConfig) *ColdStart {
	if instruments == nil {
		panic("instruments must not be nil")
	}
	return &ColdStart{
		next:         next,
		readyCache:   readyCache,
		reader:       reader,
		queueCounter: queueCounter,
		instruments:  instruments,
		cfg:          cfg,
	}
}

var _ http.Handler = (*ColdStart)(nil)

func (cs *ColdStart) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ir := util.InterceptorRouteFromContext(r.Context())

	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
	if cs.readyCache.HasReadyEndpoints(serviceKey) {
		cs.next.ServeHTTP(w, r)
		return
	}

	spec := ir.Spec.ColdStart
	placeholder := placeholderResponse(spec)

	// Routes that don't hold requests get the placeholder immediately.
	if placeholder != nil && !holdsRequests(ir) {
		serveStaticResponse(w, r, cs.reader, ir, placeholder, http.StatusServiceUnavailable)
		return
	}

	if cs.overLimit(ir) {
		cs.instruments.RecordColdStartRejection(ir.Name, ir.Namespace)
		if placeholder != nil && spec.Overflow == httpv1beta1.ColdStartOverflowPlaceholder {
			serveStaticResponse(w, r, cs.reader, ir, placeholder, http.StatusServiceUnavailable)
		} else {
			http.Error(w, "too many pending requests", http.StatusServiceUnavailable)
		}
		return
	}

	cs.next.ServeHTTP(w, r)
}

// overLimit reports whether the route has reached its pending request limit.
// The Counting middleware increments the counter before this middleware runs,
// so the count already includes the current request and held requests stay
// at or below the limit.
func (cs *ColdStart) overLimit(ir *httpv1beta1.InterceptorRoute) bool {
	limit := cs.cfg.MaxPendingRequests
	if spec := ir.Spec.ColdStart; spec != nil && spec.MaxPendingRequests != nil {
		limit = int(*spec.MaxPendingRequests)
	}
	if limit <= 0 {
		return false
	}

	key := k8s.ResourceKey(ir.Namespace, ir.Name)
	return cs.queueCounter.Concurrency(key) > limit
}

// holdsRequests reports whether the route holds cold-start requests (bounded
// by maxPendingRequests) instead of serving the placeholder immediately.
// Holding is opt-in via an explicit maxPendingRequests with
// overflow: Placeholder; overflowing requests receive the placeholder.
func holdsRequests(ir *httpv1beta1.InterceptorRoute) bool {
	spec := ir.Spec.ColdStart
	return spec != nil &&
		spec.MaxPendingRequests != nil &&
		spec.Overflow == httpv1beta1.ColdStartOverflowPlaceholder &&
		placeholderResponse(spec) != nil
}

// placeholderResponse returns the configured placeholder response, or nil.
func placeholderResponse(spec *httpv1beta1.ColdStartSpec) *httpv1beta1.StaticResponse {
	if spec == nil || spec.Placeholder == nil {
		return nil
	}
	return spec.Placeholder.Response
}
