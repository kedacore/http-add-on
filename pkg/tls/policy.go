package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Policy holds TLS security posture settings (min version, ciphers, etc.)
// rather than per-listener wiring (cert paths, ports).
type Policy struct {
	MinVersion       string `env:"KEDA_HTTP_TLS_MIN_VERSION" envDefault:""`
	MaxVersion       string `env:"KEDA_HTTP_TLS_MAX_VERSION" envDefault:""`
	CipherSuites     string `env:"KEDA_HTTP_TLS_CIPHER_SUITES" envDefault:""`
	CurvePreferences string `env:"KEDA_HTTP_TLS_CURVE_PREFERENCES" envDefault:""`
	SkipVerify       bool   `env:"KEDA_HTTP_TLS_SKIP_VERIFY" envDefault:"false"`
	// CADirs is a comma separated list of directories containing PEM-encoded
	// CA certificates to trust for outbound TLS connections to backends, in
	// addition to the system CA pool.
	CADirs string `env:"KEDA_HTTP_TLS_CA_DIRS" envDefault:""`
}

// MustParsePolicy parses TLS policy from environment variables.
func MustParsePolicy() Policy {
	return env.Must(env.ParseAs[Policy]())
}

// NewConfig returns a tls.Config with version, cipher suite, and curve
// preference fields set according to the policy. Fields left empty in the
// policy preserve Go's defaults.
func (p Policy) NewConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: p.SkipVerify, //nolint:gosec // G402: user-configurable
	}
	if p.MinVersion != "" {
		v, err := parseTLSVersion(p.MinVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS min version %q: %w", p.MinVersion, err)
		}
		cfg.MinVersion = v
	}
	if p.MaxVersion != "" {
		v, err := parseTLSVersion(p.MaxVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS max version %q: %w", p.MaxVersion, err)
		}
		cfg.MaxVersion = v
	}
	if p.CipherSuites != "" {
		suites, err := parseCipherSuites(p.CipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS cipher suites: %w", err)
		}
		cfg.CipherSuites = suites
	}
	if p.CurvePreferences != "" {
		curves, err := parseCurvePreferences(p.CurvePreferences)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS curve preferences: %w", err)
		}
		cfg.CurvePreferences = curves
	}
	return cfg, nil
}

// parseTLSVersion converts a version string to the corresponding crypto/tls
// constant. Accepts both short form ("1.2", "1.3") and the format used by
// KEDA and the operator ("TLS12", "TLS13"). Matching is case-insensitive.
func parseTLSVersion(v string) (uint16, error) {
	switch strings.ToLower(v) {
	case "1.2", "tls12":
		return tls.VersionTLS12, nil
	case "1.3", "tls13":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q: must be \"1.2\"/\"TLS12\" or \"1.3\"/\"TLS13\"", v)
	}
}

// parseCipherSuites parses a comma-separated list of TLS cipher-suite names
// into a slice of cipher-suite IDs. Returns nil when no valid names are present
// so that Go's default cipher suites remain in effect.
func parseCipherSuites(s string) ([]uint16, error) {
	lookup := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		lookup[cs.Name] = cs.ID
	}

	parts := strings.Split(s, ",")
	var suites []uint16
	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, ok := lookup[name]
		if !ok {
			return nil, fmt.Errorf("unknown cipher suite %q", name)
		}
		suites = append(suites, id)
	}
	return suites, nil
}

// parseCurvePreferences parses a comma-separated list of elliptic-curve names
// into a slice of tls.CurveID values. Both Go constant names (CurveP256)
// and standard names (P-256) are accepted. Returns nil when no valid names
// are present so that Go's default curve preferences remain in effect.
func parseCurvePreferences(s string) ([]tls.CurveID, error) {
	parts := strings.Split(s, ",")
	var curves []tls.CurveID
	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, ok := curvesByName[name]
		if !ok {
			return nil, fmt.Errorf("unknown curve %q", name)
		}
		curves = append(curves, id)
	}
	return curves, nil
}

var curvesByName = map[string]tls.CurveID{
	"CurveP256":      tls.CurveP256,
	"CurveP384":      tls.CurveP384,
	"CurveP521":      tls.CurveP521,
	"X25519":         tls.X25519,
	"X25519MLKEM768": tls.X25519MLKEM768,
	"P-256":          tls.CurveP256,
	"P-384":          tls.CurveP384,
	"P-521":          tls.CurveP521,
}
