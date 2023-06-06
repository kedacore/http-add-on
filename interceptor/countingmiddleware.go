package main

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

type CountingMiddleware struct {
	queueCounter    queue.Counter
	upstreamHandler http.Handler
}

func NewCountingMiddleware(queueCounter queue.Counter, upstreamHandler http.Handler) *CountingMiddleware {
	return &CountingMiddleware{
		queueCounter:    queueCounter,
		upstreamHandler: upstreamHandler,
	}
}

var _ http.Handler = (*CountingMiddleware)(nil)

func (cm *CountingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer cm.countAsync(ctx)()

	cm.upstreamHandler.ServeHTTP(w, r)
}

func (cm *CountingMiddleware) countAsync(ctx context.Context) func() {
	signaler := util.NewSignaler()

	go cm.count(ctx, signaler)

	return func() {
		go signaler.Signal()
	}
}

func (cm *CountingMiddleware) count(ctx context.Context, signaler util.Signaler) {
	logger, _ := ctx.Value(ContextKeyLogger).(logr.Logger)
	logger = logger.WithName("CountingMiddleware")

	httpso := ctx.Value(ContextKeyHTTPSO).(*httpv1alpha1.HTTPScaledObject)
	key := k8s.NamespacedNameFromObject(httpso).String()

	if !cm.inc(logger, key) {
		return
	}

	if err := signaler.Wait(ctx); err != nil && err != context.Canceled {
		logger.Error(err, "failed to wait signal")
	}

	cm.dec(logger, key)
}

func (cm *CountingMiddleware) inc(logger logr.Logger, key string) bool {
	if err := cm.queueCounter.Resize(key, +1); err != nil {
		logger.Error(err, "error incrementing queue counter", "key", key)

		return false
	}

	return true
}

func (cm *CountingMiddleware) dec(logger logr.Logger, key string) bool {
	if err := cm.queueCounter.Resize(key, -1); err != nil {
		logger.Error(err, "error decrementing queue counter", "key", key)

		return false
	}

	return true
}
