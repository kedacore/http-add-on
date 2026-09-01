package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-logr/logr/funcr"

	"github.com/kedacore/http-add-on/pkg/util"
)

var errUnhealthy = errors.New("unhealthy")

func TestProbeServeHTTP(t *testing.T) {
	checker := func(err error) util.HealthChecker {
		return util.HealthCheckerFunc(func(_ context.Context) error {
			return err
		})
	}

	tests := map[string]struct {
		checkers []util.HealthChecker
		code     int
	}{
		"all checks pass": {
			checkers: []util.HealthChecker{checker(nil), checker(nil), checker(nil)},
			code:     http.StatusOK,
		},
		"single check fails": {
			checkers: []util.HealthChecker{checker(errUnhealthy)},
			code:     http.StatusServiceUnavailable,
		},
		"one check among many fails": {
			checkers: []util.HealthChecker{checker(nil), checker(errUnhealthy), checker(nil)},
			code:     http.StatusServiceUnavailable,
		},
		"no checkers": {
			code: http.StatusOK,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var draining atomic.Bool
			ph := NewProbe(&draining, tt.checkers...)
			ph.check(t.Context())
			assertProbe(t, ph, tt.code)
		})
	}
}

func TestProbeHealthyToUnhealthyTransition(t *testing.T) {
	ctx := t.Context()

	var (
		draining atomic.Bool
		retErr   error
	)
	ph := NewProbe(&draining, util.HealthCheckerFunc(func(_ context.Context) error {
		return retErr
	}))

	ph.check(ctx)
	assertProbe(t, ph, http.StatusOK)

	retErr = errUnhealthy
	ph.check(ctx)
	assertProbe(t, ph, http.StatusServiceUnavailable)
}

// TestProbeFailureLogging pins down that a check which keeps failing does not
// log on every cycle: check runs once a second and a stalled routing table
// refresh keeps failing indefinitely.
func TestProbeFailureLogging(t *testing.T) {
	var lines int
	logger := funcr.New(func(_, _ string) { lines++ }, funcr.Options{})
	ctx := util.ContextWithLogger(t.Context(), logger)

	var (
		draining atomic.Bool
		retErr   error
	)
	ph := NewProbe(&draining, util.HealthCheckerFunc(func(_ context.Context) error {
		return retErr
	}))

	ph.check(ctx)
	if lines != 0 {
		t.Fatalf("a passing check logged %d lines, want 0", lines)
	}

	// Onset of the failure is logged, the repeats are not.
	retErr = errUnhealthy
	for range failureLogInterval - 1 {
		ph.check(ctx)
	}
	if lines != 1 {
		t.Errorf("%d consecutive failures logged %d lines, want 1", failureLogInterval-1, lines)
	}

	// It is logged again once the interval elapses, so a persistent failure
	// stays visible in recent logs.
	ph.check(ctx)
	if lines != 2 {
		t.Errorf("%d consecutive failures logged %d lines, want 2", failureLogInterval, lines)
	}

	// Recovery is logged once, and re-arms the onset log for the next failure.
	retErr = nil
	ph.check(ctx)
	if lines != 3 {
		t.Errorf("recovery logged %d lines in total, want 3", lines)
	}

	retErr = errUnhealthy
	ph.check(ctx)
	if lines != 4 {
		t.Errorf("failure after recovery logged %d lines in total, want 4", lines)
	}
}

func TestProbePeriodicCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			draining atomic.Bool
			count    atomic.Int64
		)
		ph := NewProbe(&draining, util.HealthCheckerFunc(func(_ context.Context) error {
			count.Add(1)
			return nil
		}))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go ph.Start(ctx)

		// Wait for Start to run the first check
		synctest.Wait()
		if got := count.Load(); got != 1 {
			t.Fatalf("after 1st cycle: count=%d, want 1", got)
		}

		// Advance the fake clock past the tick and wait for the second check to complete
		time.Sleep(time.Second)
		synctest.Wait()
		if got := count.Load(); got != 2 {
			t.Fatalf("after 2nd cycle: count=%d, want 2", got)
		}
	})
}

func TestProbeDraining(t *testing.T) {
	var draining atomic.Bool
	ph := NewProbe(&draining, util.HealthCheckerFunc(func(_ context.Context) error {
		return nil
	}))

	// Initially healthy after a check.
	ph.check(t.Context())
	assertProbe(t, ph, http.StatusOK)

	// After setting draining, probe must return 503.
	draining.Store(true)
	assertProbe(t, ph, http.StatusServiceUnavailable)

	// A subsequent health check must NOT flip readiness back to healthy.
	ph.check(t.Context())
	assertProbe(t, ph, http.StatusServiceUnavailable)
}

func assertProbe(t *testing.T, ph *Probe, code int) {
	t.Helper()
	w := httptest.NewRecorder()
	ph.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != code {
		t.Errorf("got status %d, want %d", w.Code, code)
	}
	if want := http.StatusText(code); w.Body.String() != want {
		t.Errorf("got body %q, want %q", w.Body.String(), want)
	}
}
