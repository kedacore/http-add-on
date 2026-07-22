package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/go-logr/logr"
)

// TLSPolicy holds TLS policy settings that apply across all components.
// These are security posture decisions (min version, ciphers, etc.) rather
// than per-listener wiring (cert paths, ports).
type TLSPolicy struct {
	MinVersion       string `env:"KEDA_HTTP_TLS_MIN_VERSION" envDefault:""`
	MaxVersion       string `env:"KEDA_HTTP_TLS_MAX_VERSION" envDefault:""`
	CipherSuites     string `env:"KEDA_HTTP_TLS_CIPHER_SUITES" envDefault:""`
	CurvePreferences string `env:"KEDA_HTTP_TLS_CURVE_PREFERENCES" envDefault:""`
	SkipVerify       bool   `env:"KEDA_HTTP_TLS_SKIP_VERIFY" envDefault:"false"`
}

// deprecatedTLSPolicy holds the old KEDA_HTTP_PROXY_TLS_* env vars.
// When set, they take precedence over the new names.
// TODO(v1): remove this struct and the fallback logic in MustParseTLSPolicy.
type deprecatedTLSPolicy struct {
	MinVersion       string `env:"KEDA_HTTP_PROXY_TLS_MIN_VERSION"`
	MaxVersion       string `env:"KEDA_HTTP_PROXY_TLS_MAX_VERSION"`
	CipherSuites     string `env:"KEDA_HTTP_PROXY_TLS_CIPHER_SUITES"`
	CurvePreferences string `env:"KEDA_HTTP_PROXY_TLS_CURVE_PREFERENCES"`
	SkipVerify       *bool  `env:"KEDA_HTTP_PROXY_TLS_SKIP_VERIFY"`
}

// MustParseTLSPolicy parses TLS policy from environment variables.
// Deprecated env vars take precedence over new ones when set, to preserve
// existing behavior for users who haven't migrated yet.
func MustParseTLSPolicy(log logr.Logger) TLSPolicy {
	cfg := env.Must(env.ParseAs[TLSPolicy]())

	deprecated := env.Must(env.ParseAs[deprecatedTLSPolicy]())

	if deprecated.MinVersion != "" {
		log.Info("WARNING: KEDA_HTTP_PROXY_TLS_MIN_VERSION is deprecated, use KEDA_HTTP_TLS_MIN_VERSION instead")
		cfg.MinVersion = deprecated.MinVersion
	}

	if deprecated.MaxVersion != "" {
		log.Info("WARNING: KEDA_HTTP_PROXY_TLS_MAX_VERSION is deprecated, use KEDA_HTTP_TLS_MAX_VERSION instead")
		cfg.MaxVersion = deprecated.MaxVersion
	}

	if deprecated.CipherSuites != "" {
		log.Info("WARNING: KEDA_HTTP_PROXY_TLS_CIPHER_SUITES is deprecated, use KEDA_HTTP_TLS_CIPHER_SUITES instead")
		cfg.CipherSuites = deprecated.CipherSuites
	}

	if deprecated.CurvePreferences != "" {
		log.Info("WARNING: KEDA_HTTP_PROXY_TLS_CURVE_PREFERENCES is deprecated, use KEDA_HTTP_TLS_CURVE_PREFERENCES instead")
		cfg.CurvePreferences = deprecated.CurvePreferences
	}

	if deprecated.SkipVerify != nil {
		log.Info("WARNING: KEDA_HTTP_PROXY_TLS_SKIP_VERIFY is deprecated, use KEDA_HTTP_TLS_SKIP_VERIFY instead")
		cfg.SkipVerify = *deprecated.SkipVerify
	}

	return cfg
}
