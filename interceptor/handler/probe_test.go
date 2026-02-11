package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

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
			ph := NewProbe(tt.checkers...)
			ph.check(t.Context())
			assertProbe(t, ph, tt.code)
		})
	}
}

func TestProbeHealthyToUnhealthyTransition(t *testing.T) {
	ctx := t.Context()

	var retErr error
	ph := NewProbe(util.HealthCheckerFunc(func(_ context.Context) error {
		return retErr
	}))

	ph.check(ctx)
	assertProbe(t, ph, http.StatusOK)

	retErr = errUnhealthy
	ph.check(ctx)
	assertProbe(t, ph, http.StatusServiceUnavailable)
}

func TestProbePeriodicCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count int
		ph := NewProbe(util.HealthCheckerFunc(func(_ context.Context) error {
			count++
			return nil
		}))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go ph.Start(ctx)

		// Wait for Start to run the first check
		synctest.Wait()
		if count != 1 {
			t.Fatalf("after 1st cycle: count=%d, want 1", count)
		}

		// Advance the fake clock past the tick and wait for the second check to complete
		time.Sleep(time.Second)
		synctest.Wait()
		if count != 2 {
			t.Fatalf("after 2nd cycle: count=%d, want 2", count)
		}
	})
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
