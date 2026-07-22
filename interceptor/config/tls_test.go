package config

import (
	"testing"

	"github.com/go-logr/logr"
)

func TestMustParseTLSPolicy_DeprecatedOverride(t *testing.T) {
	t.Setenv("KEDA_HTTP_TLS_MIN_VERSION", "new-min")
	t.Setenv("KEDA_HTTP_TLS_MAX_VERSION", "new-max")
	t.Setenv("KEDA_HTTP_TLS_CIPHER_SUITES", "new-cipher")
	t.Setenv("KEDA_HTTP_TLS_CURVE_PREFERENCES", "new-curve")
	t.Setenv("KEDA_HTTP_TLS_SKIP_VERIFY", "true")

	t.Setenv("KEDA_HTTP_PROXY_TLS_MIN_VERSION", "deprecated-min")
	t.Setenv("KEDA_HTTP_PROXY_TLS_MAX_VERSION", "deprecated-max")
	t.Setenv("KEDA_HTTP_PROXY_TLS_CIPHER_SUITES", "deprecated-cipher")
	t.Setenv("KEDA_HTTP_PROXY_TLS_CURVE_PREFERENCES", "deprecated-curve")
	t.Setenv("KEDA_HTTP_PROXY_TLS_SKIP_VERIFY", "false")

	cfg := MustParseTLSPolicy(logr.Discard())

	if cfg.MinVersion != "deprecated-min" {
		t.Errorf("MinVersion = %q, want %q (deprecated var should take precedence)", cfg.MinVersion, "deprecated-min")
	}
	if cfg.MaxVersion != "deprecated-max" {
		t.Errorf("MaxVersion = %q, want %q (deprecated var should take precedence)", cfg.MaxVersion, "deprecated-max")
	}
	if cfg.CipherSuites != "deprecated-cipher" {
		t.Errorf("CipherSuites = %q, want %q (deprecated var should take precedence)", cfg.CipherSuites, "deprecated-cipher")
	}
	if cfg.CurvePreferences != "deprecated-curve" {
		t.Errorf("CurvePreferences = %q, want %q (deprecated var should take precedence)", cfg.CurvePreferences, "deprecated-curve")
	}
	if cfg.SkipVerify {
		t.Error("SkipVerify = true, want false (deprecated var should take precedence)")
	}
}
