package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
)

type HealthCheck func(ctx context.Context) error

type ProbeHandler struct {
	healthChecks []HealthCheck
	healthy      atomic.Bool
}

func NewProbeHandler(healthChecks []HealthCheck) *ProbeHandler {
	return &ProbeHandler{
		healthChecks: healthChecks,
	}
}

var _ http.Handler = (*ProbeHandler)(nil)

func (ph *ProbeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger, _ := ctx.Value(ContextKeyLogger).(logr.Logger)
	logger = logger.WithName("ProbeHandler")

	sc := http.StatusOK
	if !ph.healthy.Load() {
		sc = http.StatusServiceUnavailable
	}
	w.WriteHeader(sc)

	st := http.StatusText(sc)
	if _, err := w.Write([]byte(st)); err != nil {
		logger.Error(err, "write failed")
	}
}

func (ph *ProbeHandler) Start(ctx context.Context) {
	for {
		ph.check(ctx)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
			continue
		}
	}
}

func (ph *ProbeHandler) check(ctx context.Context) {
	logger, _ := ctx.Value(ContextKeyLogger).(logr.Logger)
	logger = logger.WithName("ProbeHandler")

	for _, hc := range ph.healthChecks {
		if err := hc(ctx); err != nil {
			ph.healthy.Store(false)

			logger.Error(err, "health check function failed")
			return
		}
	}

	ph.healthy.Store(true)
}
