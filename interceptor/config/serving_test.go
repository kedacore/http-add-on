package config

import (
	"testing"
	"time"
)

func TestMustParseServing_RoutingTableDefaults(t *testing.T) {
	t.Setenv("KEDA_HTTP_PROXY_PORT", "8080")
	t.Setenv("KEDA_HTTP_ADMIN_PORT", "9090")

	cfg := MustParseServing()

	if got, want := cfg.RoutingTableRefreshInterval, time.Minute; got != want {
		t.Errorf("RoutingTableRefreshInterval = %v, want %v", got, want)
	}
	if got, want := cfg.RoutingTableMaxStaleness, 5*time.Minute; got != want {
		t.Errorf("RoutingTableMaxStaleness = %v, want %v", got, want)
	}
}

func TestServingValidateRoutingTable(t *testing.T) {
	tests := map[string]struct {
		refreshInterval time.Duration
		maxStaleness    time.Duration
		wantErr         bool
	}{
		"defaults": {
			refreshInterval: time.Minute,
			maxStaleness:    5 * time.Minute,
		},
		"staleness check disabled": {
			refreshInterval: time.Minute,
		},
		"both disabled": {},
		"staleness exactly twice the interval": {
			refreshInterval: time.Minute,
			maxStaleness:    2 * time.Minute,
		},
		// Nothing advances the rebuild time, so an interceptor with no route
		// changes would fail readiness as soon as the deadline elapsed.
		"staleness without periodic rebuild": {
			maxStaleness: 5 * time.Minute,
			wantErr:      true,
		},
		// A single missed rebuild would fail readiness.
		"staleness equal to the interval": {
			refreshInterval: time.Minute,
			maxStaleness:    time.Minute,
			wantErr:         true,
		},
		"negative interval": {
			refreshInterval: -time.Minute,
			wantErr:         true,
		},
		"negative staleness": {
			refreshInterval: time.Minute,
			maxStaleness:    -time.Minute,
			wantErr:         true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := Serving{
				RoutingTableRefreshInterval: tc.refreshInterval,
				RoutingTableMaxStaleness:    tc.maxStaleness,
			}

			err := cfg.validate()
			if tc.wantErr && err == nil {
				t.Error("expected a validation error, got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no validation error, got: %v", err)
			}
		})
	}
}
