package handler

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kedacore/http-add-on/pkg/util"
)

// failureLogInterval is how many consecutive failing checks pass between log
// lines while a check keeps failing. check runs once a second, so a failure that
// persists is logged on onset and then about every five minutes, instead of
// every second for as long as it lasts.
const failureLogInterval = 300

type Probe struct {
	draining       *atomic.Bool
	hasBeenHealthy atomic.Bool
	healthCheckers []util.HealthChecker
	healthy        atomic.Bool

	// consecutiveFailures is only touched by check, which runs on a single
	// goroutine.
	consecutiveFailures int
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
			ph.consecutiveFailures++

			if ph.consecutiveFailures == 1 || ph.consecutiveFailures%failureLogInterval == 0 {
				// Log at info level before the first successful check to avoid
				// noisy error logs during normal startup sequencing.
				if ph.hasBeenHealthy.Load() {
					logger.Error(err, "health check failed", "consecutiveFailures", ph.consecutiveFailures)
				} else {
					logger.Info("waiting for health check to pass", "error", err, "consecutiveFailures", ph.consecutiveFailures)
				}
			}
			return
		}
	}

	if ph.consecutiveFailures > 0 {
		logger.Info("health check passing again", "consecutiveFailures", ph.consecutiveFailures)
		ph.consecutiveFailures = 0
	}

	ph.healthy.Store(true)
	ph.hasBeenHealthy.Store(true)
}
