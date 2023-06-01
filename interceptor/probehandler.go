package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
)

type HealthCheck func() error

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

	logger := ctx.Value(LoggerContextKey).(logr.Logger)
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

func (ph *ProbeHandler) Start(ctx context.Context, logger logr.Logger) {
	for {
		ph.check(logger)

		select {
		case <-time.After(time.Second):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (ph *ProbeHandler) check(logger logr.Logger) {
	for _, hc := range ph.healthChecks {
		if err := hc(); err != nil {
			ph.healthy.Store(false)

			logger.Error(err, "health check function failed")
			return
		}
	}

	ph.healthy.Store(true)
}
