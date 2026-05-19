package handler

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kedacore/http-add-on/pkg/util"
)

type Probe struct {
	draining       *atomic.Bool
	hasBeenHealthy atomic.Bool
	healthCheckers []util.HealthChecker
	healthy        atomic.Bool
}

func NewProbe(draining *atomic.Bool, healthChecks ...util.HealthChecker) *Probe {
	return &Probe{
		draining:       draining,
		healthCheckers: healthChecks,
	}
}

var _ http.Handler = (*Probe)(nil)

func (ph *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "ProbeHandler")
	ctx := r.Context()

	sc := http.StatusOK
	if ph.draining.Load() || !ph.healthy.Load() {
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
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		ph.check(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			continue
		}
	}
}

func (ph *Probe) check(ctx context.Context) {
	if ph.draining.Load() {
		return
	}

	logger := util.LoggerFromContext(ctx)
	logger = logger.WithName("Probe")

	for _, hc := range ph.healthCheckers {
		if err := hc.HealthCheck(ctx); err != nil {
			ph.healthy.Store(false)

			// Log at info level before the first successful check to avoid
			// noisy error logs during normal startup sequencing.
			if ph.hasBeenHealthy.Load() {
				logger.Error(err, "health check failed")
			} else {
				logger.Info("waiting for health check to pass", "error", err)
			}
			return
		}
	}

	ph.healthy.Store(true)
	ph.hasBeenHealthy.Store(true)
}
