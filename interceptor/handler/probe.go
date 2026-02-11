package handler

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kedacore/http-add-on/pkg/util"
)

type Probe struct {
	healthCheckers []util.HealthChecker
	healthy        atomic.Bool
	hasBeenHealthy atomic.Bool
}

func NewProbe(healthChecks ...util.HealthChecker) *Probe {
	return &Probe{
		healthCheckers: healthChecks,
	}
}

var _ http.Handler = (*Probe)(nil)

func (ph *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "ProbeHandler")
	ctx := r.Context()

	sc := http.StatusOK
	if !ph.healthy.Load() {
		sc = http.StatusServiceUnavailable
	}
	w.WriteHeader(sc)

	st := http.StatusText(sc)
	if _, err := w.Write([]byte(st)); err != nil {
		logger := util.LoggerFromContext(ctx)
		logger.Error(err, "write failed")
	}
}

func (ph *Probe) Start(ctx context.Context) {
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

func (ph *Probe) check(ctx context.Context) {
	logger := util.LoggerFromContext(ctx)
	logger = logger.WithName("Probe")

	for _, hc := range ph.healthCheckers {
		if err := hc.HealthCheck(ctx); err != nil {
			ph.healthy.Store(false)

			// Log at info level before the first successful check to avoid
			// noisy error logs during normal startup sequencing.
			if ph.hasBeenHealthy.Load() {
				logger.Error(err, "health check function failed")
			} else {
				logger.Info("waiting for health check to pass", "error", err)
			}
			return
		}
	}

	ph.healthy.Store(true)
	ph.hasBeenHealthy.Store(true)
}
