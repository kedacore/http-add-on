package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-logr/logr"

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

	defer cm.countAsync(ctx)()

	cm.upstreamHandler.ServeHTTP(w, r)
}

func (cm *Counting) countAsync(ctx context.Context) func() {
	signaler := util.NewSignaler()

	go cm.count(ctx, signaler)

	return func() {
		go signaler.Signal()
	}
}

func (cm *Counting) count(ctx context.Context, signaler util.Signaler) {
	logger := util.LoggerFromContext(ctx)
	httpso := util.HTTPSOFromContext(ctx)

	key := k8s.NamespacedNameFromObject(httpso).String()

	if !cm.inc(logger, key) {
		return
	}

	if err := signaler.Wait(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error(err, "failed to wait signal")
	}

	cm.dec(logger, key)
}

func (cm *Counting) inc(logger logr.Logger, key string) bool {
	if err := cm.queueCounter.Increase(key, 1); err != nil {
		logger.Error(err, "error incrementing queue counter", "key", key)

		return false
	}

	metrics.RecordPendingRequestCount(key, int64(1))

	return true
}

func (cm *Counting) dec(logger logr.Logger, key string) bool {
	if err := cm.queueCounter.Decrease(key, 1); err != nil {
		logger.Error(err, "error decrementing queue counter", "key", key)

		return false
	}

	metrics.RecordPendingRequestCount(key, int64(-1))

	return true
}
