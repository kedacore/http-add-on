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
	instruments  *metrics.Instruments
}

func NewCounting(next http.Handler, queueCounter queue.Counter, instruments *metrics.Instruments) *Counting {
	if instruments == nil {
		panic("instruments must not be nil")
	}
	return &Counting{
		next:         next,
		queueCounter: queueCounter,
		instruments:  instruments,
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
	cm.instruments.RecordPendingRequest(ir.Name, ir.Namespace, 1)

	defer func() {
		if err := cm.queueCounter.Decrease(key, 1); err != nil {
			util.LoggerFromContext(ctx).Error(err, "error decrementing queue counter", "key", key)
		}
		cm.instruments.RecordPendingRequest(ir.Name, ir.Namespace, -1)
	}()

	cm.next.ServeHTTP(w, r)
}
