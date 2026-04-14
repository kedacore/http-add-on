package config

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestMustParseTimeouts_DeprecatedOverride(t *testing.T) {
	t.Setenv("KEDA_HTTP_READINESS_TIMEOUT", "25s")
	t.Setenv("KEDA_CONDITION_WAIT_TIMEOUT", "31s")
	t.Setenv("KEDA_RESPONSE_HEADER_TIMEOUT", "7s")

	cfg := MustParseTimeouts(logr.Discard())

	if got, want := cfg.Readiness, 31*time.Second; got != want {
		t.Errorf("Readiness = %v, want %v (deprecated var should take precedence)", got, want)
	}
	if got, want := cfg.ResponseHeader, 7*time.Second; got != want {
		t.Errorf("ResponseHeader = %v, want %v (deprecated var should take precedence)", got, want)
	}
}
