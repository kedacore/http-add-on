package tls_test

import (
	"crypto/tls"
	"slices"
	"testing"

	kedatls "github.com/kedacore/http-add-on/pkg/tls"
)

func TestPolicyNewConfig(t *testing.T) {
	tests := map[string]struct {
		policy           kedatls.Policy
		wantErr          bool
		wantMinVersion   uint16
		wantMaxVersion   uint16
		wantCipherSuites []uint16
		wantCurves       []tls.CurveID
	}{
		"empty policy": {
			policy:         kedatls.Policy{},
			wantMinVersion: 0,
		},
		"min version 1.3": {
			policy:         kedatls.Policy{MinVersion: "1.3"},
			wantMinVersion: tls.VersionTLS13,
		},
		"min version 1.2": {
			policy:         kedatls.Policy{MinVersion: "1.2"},
			wantMinVersion: tls.VersionTLS12,
		},
		"min version TLS12": {
			policy:         kedatls.Policy{MinVersion: "TLS12"},
			wantMinVersion: tls.VersionTLS12,
		},
		"min version tls12 lowercase": {
			policy:         kedatls.Policy{MinVersion: "tls12"},
			wantMinVersion: tls.VersionTLS12,
		},
		"min version TLS13": {
			policy:         kedatls.Policy{MinVersion: "TLS13"},
			wantMinVersion: tls.VersionTLS13,
		},
		"max version 1.2": {
			policy:         kedatls.Policy{MaxVersion: "1.2"},
			wantMaxVersion: tls.VersionTLS12,
		},
		"max version TLS12": {
			policy:         kedatls.Policy{MaxVersion: "TLS12"},
			wantMaxVersion: tls.VersionTLS12,
		},
		"max version tls12 lowercase": {
			policy:         kedatls.Policy{MaxVersion: "tls12"},
			wantMaxVersion: tls.VersionTLS12,
		},
		"max version TLS13": {
			policy:         kedatls.Policy{MaxVersion: "TLS13"},
			wantMaxVersion: tls.VersionTLS13,
		},
		"invalid min version": {
			policy:  kedatls.Policy{MinVersion: "1.1"},
			wantErr: true,
		},
		"invalid max version": {
			policy:  kedatls.Policy{MaxVersion: "1.0"},
			wantErr: true,
		},
		"cipher suites": {
			policy: kedatls.Policy{CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			wantCipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
		"cipher suites whitespace only": {
			policy:           kedatls.Policy{CipherSuites: " , "},
			wantCipherSuites: nil,
		},
		"invalid cipher suite": {
			policy:  kedatls.Policy{CipherSuites: "INVALID_SUITE"},
			wantErr: true,
		},
		"curve preferences go names": {
			policy:     kedatls.Policy{CurvePreferences: "X25519,CurveP256"},
			wantCurves: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
		"curve preferences standard names": {
			policy:     kedatls.Policy{CurvePreferences: "P-256, P-384"},
			wantCurves: []tls.CurveID{tls.CurveP256, tls.CurveP384},
		},
		"curve preferences whitespace only": {
			policy:     kedatls.Policy{CurvePreferences: " , "},
			wantCurves: nil,
		},
		"invalid curve preference": {
			policy:  kedatls.Policy{CurvePreferences: "INVALID_CURVE"},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := tt.policy.NewConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MinVersion != tt.wantMinVersion {
				t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tt.wantMinVersion)
			}
			if cfg.MaxVersion != tt.wantMaxVersion {
				t.Errorf("MaxVersion = %d, want %d", cfg.MaxVersion, tt.wantMaxVersion)
			}
			if !slices.Equal(cfg.CipherSuites, tt.wantCipherSuites) {
				t.Errorf("CipherSuites = %v, want %v", cfg.CipherSuites, tt.wantCipherSuites)
			}
			if !slices.Equal(cfg.CurvePreferences, tt.wantCurves) {
				t.Errorf("CurvePreferences = %v, want %v", cfg.CurvePreferences, tt.wantCurves)
			}
		})
	}
}
