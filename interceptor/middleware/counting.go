package middleware

import (
	"net/http"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

type Counting struct {
	queueCounter    queue.Counter
	upstreamHandler http.Handler
}

func NewCountingMiddleware(queueCounter queue.Counter, upstreamHandler http.Handler) *Counting {
	return &Counting{
		queueCounter:    queueCounter,
		upstreamHandler: upstreamHandler,
	}
}

var _ http.Handler = (*Counting)(nil)

func (cm *Counting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "CountingMiddleware")
	ctx := r.Context()
	httpso := util.HTTPSOFromContext(ctx)

	key := k8s.NamespacedNameFromObject(httpso).String()

	if err := cm.queueCounter.Increase(key, 1); err != nil {
		util.LoggerFromContext(ctx).Error(err, "error incrementing queue counter", "key", key)
		cm.upstreamHandler.ServeHTTP(w, r)
		return
	}
	metrics.RecordPendingRequestCount(key, 1)

	defer func() {
		if err := cm.queueCounter.Decrease(key, 1); err != nil {
			util.LoggerFromContext(ctx).Error(err, "error decrementing queue counter", "key", key)
		}
		metrics.RecordPendingRequestCount(key, -1)
	}()

	cm.upstreamHandler.ServeHTTP(w, r)
}
