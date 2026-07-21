package main

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/testutil"
)

func TestBuildTLSConfig_CertificatePath(t *testing.T) {
	dir := t.TempDir()
	writeCert(t, dir, "server", "example.com")

	opts := TLSOptions{
		CertificatePath: filepath.Join(dir, "server.crt"),
		KeyPath:         filepath.Join(dir, "server.key"),
	}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "example.com")
}

func TestBuildTLSConfig_CertStorePaths(t *testing.T) {
	dir := t.TempDir()
	writeCert(t, dir, "svc1", "svc1.example.com")
	writeCert(t, dir, "svc2", "svc2.example.com")

	opts := TLSOptions{CertStorePaths: dir}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "svc1.example.com")
	requireCertForHost(t, tlsCfg, "svc2.example.com")
}

func TestBuildTLSConfig_MultipleCertStorePaths(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()
	writeCert(t, dir1, "a", "a.example.com")
	writeCert(t, dir2, "b", "b.example.com")

	opts := TLSOptions{CertStorePaths: dir1 + "," + dir2}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "a.example.com")
	requireCertForHost(t, tlsCfg, "b.example.com")
}

func TestBuildTLSConfig_FallbackToDefault(t *testing.T) {
	dir := t.TempDir()
	writeCert(t, dir, "default", "default.example.com")

	opts := TLSOptions{
		CertificatePath: filepath.Join(dir, "default.crt"),
		KeyPath:         filepath.Join(dir, "default.key"),
	}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "unknown.example.com")
}

func TestBuildTLSConfig_NoDefaultCert(t *testing.T) {
	opts := TLSOptions{}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	_, err = tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "any.com"})
	if err == nil {
		t.Error("expected error for unknown host with no default cert")
	}
}

func TestBuildTLSConfig_MissingKeyFile(t *testing.T) {
	dir := t.TempDir()
	certPEM, _ := testutil.GenerateCertPEM(t, []string{"example.com"}, nil)
	writeFile(t, filepath.Join(dir, "server.crt"), certPEM)

	opts := TLSOptions{CertStorePaths: dir}

	_, _, err := BuildTLSConfig(opts, logr.Discard())
	if err == nil {
		t.Error("expected error for missing key file")
	}
}

func TestBuildTLSConfig_PemFormat(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := testutil.GenerateCertPEM(t, []string{"pem.example.com"}, nil)
	writeFile(t, filepath.Join(dir, "server.pem"), certPEM)
	writeFile(t, filepath.Join(dir, "server-key.pem"), keyPEM)

	opts := TLSOptions{CertStorePaths: dir}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "pem.example.com")
}

func TestBuildTLSConfig_IPAddressSAN(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := testutil.GenerateCertPEM(t, nil, []net.IP{net.ParseIP("192.168.1.100")})
	writeFile(t, filepath.Join(dir, "ip.crt"), certPEM)
	writeFile(t, filepath.Join(dir, "ip.key"), keyPEM)

	opts := TLSOptions{CertStorePaths: dir}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	requireCertForHost(t, tlsCfg, "192.168.1.100")
}

func TestBuildTLSConfig_InvalidContent(t *testing.T) {
	tests := map[string]struct {
		invalidCert bool
		invalidKey  bool
	}{
		"invalid cert": {invalidCert: true},
		"invalid key":  {invalidKey: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			certPEM, keyPEM := testutil.GenerateCertPEM(t, []string{"example.com"}, nil)

			if tt.invalidCert {
				certPEM = []byte("not a valid certificate")
			}
			if tt.invalidKey {
				keyPEM = []byte("not a valid key")
			}

			writeFile(t, filepath.Join(dir, "server.crt"), certPEM)
			writeFile(t, filepath.Join(dir, "server.key"), keyPEM)

			opts := TLSOptions{CertStorePaths: dir}

			_, _, err := BuildTLSConfig(opts, logr.Discard())
			if err == nil {
				t.Error("expected error for invalid content")
			}
		})
	}
}

func TestBuildTLSConfig_NonExistentCertStorePath(t *testing.T) {
	opts := TLSOptions{CertStorePaths: "/nonexistent/path/to/certs"}

	_, _, err := BuildTLSConfig(opts, logr.Discard())
	if err == nil {
		t.Error("expected error for non-existent cert store path")
	}
}

func TestBuildTLSConfig_TLSOptions(t *testing.T) {
	tests := map[string]struct {
		opts             TLSOptions
		wantErr          bool
		wantMinVersion   uint16
		wantMaxVersion   uint16
		wantCipherSuites []uint16
		wantCurves       []tls.CurveID
	}{
		"default min version": {
			opts:           TLSOptions{},
			wantMinVersion: 0,
		},
		"min version 1.3": {
			opts:           TLSOptions{MinTLSVersion: "1.3"},
			wantMinVersion: tls.VersionTLS13,
		},
		"min version 1.2": {
			opts:           TLSOptions{MinTLSVersion: "1.2"},
			wantMinVersion: tls.VersionTLS12,
		},
		"max version 1.2": {
			opts:           TLSOptions{MaxTLSVersion: "1.2"},
			wantMaxVersion: tls.VersionTLS12,
		},
		"invalid min version": {
			opts:    TLSOptions{MinTLSVersion: "1.1"},
			wantErr: true,
		},
		"invalid max version": {
			opts:    TLSOptions{MaxTLSVersion: "1.0"},
			wantErr: true,
		},
		"cipher suites": {
			opts: TLSOptions{CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			wantCipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
		"cipher suites whitespace only": {
			opts:             TLSOptions{CipherSuites: " , "},
			wantCipherSuites: nil,
		},
		"invalid cipher suite": {
			opts:    TLSOptions{CipherSuites: "INVALID_SUITE"},
			wantErr: true,
		},
		"curve preferences go names": {
			opts:       TLSOptions{CurvePreferences: "X25519,CurveP256"},
			wantCurves: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
		"curve preferences standard names": {
			opts:       TLSOptions{CurvePreferences: "P-256, P-384"},
			wantCurves: []tls.CurveID{tls.CurveP256, tls.CurveP384},
		},
		"curve preferences whitespace only": {
			opts:       TLSOptions{CurvePreferences: " , "},
			wantCurves: nil,
		},
		"invalid curve preference": {
			opts:    TLSOptions{CurvePreferences: "INVALID_CURVE"},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tlsCfg, _, err := BuildTLSConfig(tt.opts, logr.Discard())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tlsCfg.MinVersion != tt.wantMinVersion {
				t.Errorf("MinVersion = %d, want %d", tlsCfg.MinVersion, tt.wantMinVersion)
			}
			if tlsCfg.MaxVersion != tt.wantMaxVersion {
				t.Errorf("MaxVersion = %d, want %d", tlsCfg.MaxVersion, tt.wantMaxVersion)
			}
			if !slices.Equal(tlsCfg.CipherSuites, tt.wantCipherSuites) {
				t.Errorf("CipherSuites = %v, want %v", tlsCfg.CipherSuites, tt.wantCipherSuites)
			}
			if !slices.Equal(tlsCfg.CurvePreferences, tt.wantCurves) {
				t.Errorf("CurvePreferences = %v, want %v", tlsCfg.CurvePreferences, tt.wantCurves)
			}
		})
	}
}

func TestBuildTLSConfig_CertwatcherHotReload(t *testing.T) {
	dir := t.TempDir()
	writeCert(t, dir, "server", "original.example.com")

	opts := TLSOptions{
		CertificatePath: filepath.Join(dir, "server.crt"),
		KeyPath:         filepath.Join(dir, "server.key"),
	}

	tlsCfg, watcher, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}
	go func() {
		_ = watcher.Start(t.Context())
	}()

	// Certwatcher serves as default cert for any SNI
	cert, err := tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "any.example.com"})
	if err != nil {
		t.Fatalf("expected default cert, got error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil cert")
	}

	// Overwrite cert on disk with a new SAN
	writeCert(t, dir, "server", "reloaded.example.com")

	if readErr := watcher.ReadCertificate(); readErr != nil {
		t.Fatalf("reading certificate: %v", readErr)
	}

	for _, sni := range []string{
		"any.example.com",
		"unchecked.example.com",
	} {
		cert, err = tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: sni})
		if err != nil {
			t.Fatalf("expected reloaded cert for SNI %q, got error: %v", sni, err)
		}
		if cert.Leaf == nil {
			t.Fatalf("expected parsed leaf certificate for SNI %q", sni)
		}
		if !slices.Contains(cert.Leaf.DNSNames, "reloaded.example.com") {
			t.Errorf("expected reloaded cert for SNI %q, got %v", sni, cert.Leaf.DNSNames)
		}
	}
}

func TestBuildTLSConfig_SNIPriorityOverDefault(t *testing.T) {
	defaultCertDir := t.TempDir()
	writeCert(t, defaultCertDir, "default", "default.example.com")

	sniCertDir := t.TempDir()
	writeCert(t, sniCertDir, "sni", "specific.example.com")

	opts := TLSOptions{
		CertificatePath: filepath.Join(defaultCertDir, "default.crt"),
		KeyPath:         filepath.Join(defaultCertDir, "default.key"),
		CertStorePaths:  sniCertDir,
	}

	tlsCfg, _, err := BuildTLSConfig(opts, logr.Discard())
	if err != nil {
		t.Fatalf("failed to build TLS config: %v", err)
	}

	// SNI match should return the store cert
	cert, err := tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "specific.example.com"})
	if err != nil {
		t.Fatalf("expected SNI cert, got error: %v", err)
	}
	if cert.Leaf == nil {
		t.Fatal("expected parsed leaf")
	}
	if cert.Leaf.DNSNames[0] != "specific.example.com" {
		t.Errorf("expected SNI cert for specific.example.com, got %v", cert.Leaf.DNSNames)
	}

	// Unknown SNI should fall back to certwatcher default
	cert, err = tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: "unknown.example.com"})
	if err != nil {
		t.Fatalf("expected default cert, got error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil default cert")
	}
}

func requireCertForHost(t *testing.T, cfg *tls.Config, host string) {
	t.Helper()
	cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{ServerName: host})
	if err != nil {
		t.Fatalf("no cert for %s: %v", host, err)
	}
	if cert == nil {
		t.Fatalf("no cert for %s: got nil", host)
	}
}

func writeCert(t *testing.T, dir, name, dnsName string) {
	t.Helper()
	certPEM, keyPEM := testutil.GenerateCertPEM(t, []string{dnsName}, nil)
	writeFile(t, filepath.Join(dir, name+".crt"), certPEM)
	writeFile(t, filepath.Join(dir, name+".key"), keyPEM)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
