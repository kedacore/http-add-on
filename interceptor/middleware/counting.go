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

// queueOverflowRetryAfter is the Retry-After value (in seconds) sent with the
// 503 response when a route's cold-start hold queue is full.
const queueOverflowRetryAfter = "5"

// CountingConfig configures the Counting middleware.
type CountingConfig struct {
	// MaxQueueDepth is the default cap on requests held per route while its
	// backend has no ready endpoints. Routes override it via
	// coldStart.maxQueueDepth. Zero or negative means unlimited.
	MaxQueueDepth int
}

type Counting struct {
	next         http.Handler
	queueCounter queue.Counter
	instruments  *metrics.Instruments
	readyCache   *k8s.ReadyEndpointsCache
	reader       client.Reader
	cfg          CountingConfig
}

func NewCounting(next http.Handler, queueCounter queue.Counter, instruments *metrics.Instruments, readyCache *k8s.ReadyEndpointsCache, reader client.Reader, cfg CountingConfig) *Counting {
	if instruments == nil {
		panic("instruments must not be nil")
	}
	return &Counting{
		next:         next,
		queueCounter: queueCounter,
		instruments:  instruments,
		readyCache:   readyCache,
		reader:       reader,
		cfg:          cfg,
	}
}

var _ http.Handler = (*Counting)(nil)

func (cm *Counting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "CountingMiddleware")
	ctx := r.Context()
	ir := util.InterceptorRouteFromContext(ctx)

	key := k8s.ResourceKey(ir.Namespace, ir.Name)

	if cm.holdQueueFull(ir, key) {
		cm.instruments.RecordColdStartRejection(ir.Name, ir.Namespace)
		util.LoggerFromContext(ctx).V(1).Info("cold-start hold queue full, request not admitted", "key", key)
		if overflowServesPlaceholder(ir) {
			serveStaticResponse(w, r, cm.reader, ir, ir.Spec.ColdStart.Placeholder.Response, http.StatusServiceUnavailable)
		} else {
			w.Header().Set("Retry-After", queueOverflowRetryAfter)
			http.Error(w, "cold-start hold queue full", http.StatusServiceUnavailable)
		}
		return
	}

	if err := cm.queueCounter.Increase(key, 1); err != nil {
		util.LoggerFromContext(ctx).Error(err, "error incrementing queue counter", "key", key)
		cm.next.ServeHTTP(w, r)
		return
	}
	cm.instruments.RecordPendingRequest(ir.Name, ir.Namespace, 1)

	defer func() {
		if err := cm.queueCounter.Decrease(key, 1); err != nil {
			util.LoggerFromContext(ctx).Error(err, "error decrementing queue counter", "key", key)
		}
		cm.instruments.RecordPendingRequest(ir.Name, ir.Namespace, -1)
	}()

	cm.next.ServeHTTP(w, r)
}

// holdQueueFull reports whether the route's cold-start hold queue is at
// capacity. The cap applies only while the target has no ready endpoints:
// those are the requests that park in the EndpointResolver waiting for
// scale-from-zero; warm requests flow through and need no admission control.
// The check is advisory: a concurrent burst may exceed the cap slightly
// before the counter reflects it.
func (cm *Counting) holdQueueFull(ir *httpv1beta1.InterceptorRoute, key string) bool {
	maxDepth := cm.cfg.MaxQueueDepth
	if ir.Spec.ColdStart != nil && ir.Spec.ColdStart.MaxQueueDepth != nil {
		maxDepth = int(*ir.Spec.ColdStart.MaxQueueDepth)
	}
	if maxDepth <= 0 {
		return false
	}

	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
	if cm.readyCache.HasReadyEndpoints(serviceKey) {
		return false
	}

	return cm.queueCounter.Concurrency(key) >= maxDepth
}

// overflowServesPlaceholder reports whether requests overflowing the hold
// queue should receive the route's placeholder response instead of a 503.
func overflowServesPlaceholder(ir *httpv1beta1.InterceptorRoute) bool {
	cs := ir.Spec.ColdStart
	return cs != nil &&
		cs.Overflow == httpv1beta1.ColdStartOverflowPlaceholder &&
		cs.Placeholder != nil &&
		cs.Placeholder.Response != nil
}
