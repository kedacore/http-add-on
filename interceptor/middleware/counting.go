package middleware

import (
	"net/http"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

type Counting struct {
	next         http.Handler
	queueCounter queue.Counter
}

func NewCountingMiddleware(next http.Handler, queueCounter queue.Counter) *Counting {
	return &Counting{
		next:         next,
		queueCounter: queueCounter,
	}
}

var _ http.Handler = (*Counting)(nil)

func (cm *Counting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "CountingMiddleware")
	ctx := r.Context()
	ir := util.InterceptorRouteFromContext(ctx)

	key := k8s.ResourceKey(ir.Namespace, ir.Name)

	if err := cm.queueCounter.Increase(key, 1); err != nil {
		util.LoggerFromContext(ctx).Error(err, "error incrementing queue counter", "key", key)
		cm.next.ServeHTTP(w, r)
		return
	}
	metrics.RecordPendingRequestCount(key, 1)

	defer func() {
		if err := cm.queueCounter.Decrease(key, 1); err != nil {
			util.LoggerFromContext(ctx).Error(err, "error decrementing queue counter", "key", key)
		}
		metrics.RecordPendingRequestCount(key, -1)
	}()

	cm.next.ServeHTTP(w, r)
}
