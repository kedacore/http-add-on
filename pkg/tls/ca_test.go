package tls_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/kedacore/http-add-on/pkg/testutil"
	kedatls "github.com/kedacore/http-add-on/pkg/tls"
)

func TestBuildCAPool(t *testing.T) {
	tests := map[string]struct {
		// setup prepares caDirs and returns the CA cert that should be
		// trusted by the pool, or nil if the case loads no CA.
		setup   func(t *testing.T) (caDirs string, caCertPEM []byte)
		wantErr bool
	}{
		"empty string": {
			setup: func(_ *testing.T) (string, []byte) {
				return "", nil
			},
			wantErr: false,
		},
		"single dir with one CA": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir, "ca.crt"), caCertPEM)
				return dir, caCertPEM
			},
			wantErr: false,
		},
		"multiple comma-separated dirs": {
			setup: func(t *testing.T) (string, []byte) {
				dir1 := t.TempDir()
				dir2 := t.TempDir()
				otherCACertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir1, "ca.crt"), otherCACertPEM)
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir2, "ca.crt"), caCertPEM)
				return dir1 + "," + dir2, caCertPEM
			},
			wantErr: false,
		},
		"nonexistent dir": {
			setup: func(_ *testing.T) (string, []byte) {
				return "/does/not/exist", nil
			},
			wantErr: true,
		},
		"one dir missing among multiple": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir, "ca.crt"), caCertPEM)
				return dir + ",/does/not/exist", caCertPEM
			},
			wantErr: true,
		},
		"non-PEM garbage file": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				writeFile(t, filepath.Join(dir, "garbage"), []byte("not a cert"))
				return dir, nil
			},
			wantErr: true,
		},
		"dot-dot prefixed file skipped": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir, "..data"), caCertPEM)
				return dir, nil
			},
			wantErr: true,
		},
		"subdirectory skipped": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				sub := filepath.Join(dir, "sub")
				if err := os.Mkdir(sub, 0o700); err != nil {
					t.Fatalf("creating subdir: %v", err)
				}
				writeFile(t, filepath.Join(sub, "ca.crt"), caCertPEM)
				return dir, nil
			},
			wantErr: true,
		},
		"mixed valid and invalid": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir, "ca.crt"), caCertPEM)
				writeFile(t, filepath.Join(dir, "garbage"), []byte("not a cert"))
				return dir, caCertPEM
			},
			wantErr: true,
		},
		"empty dir": {
			setup: func(t *testing.T) (string, []byte) {
				return t.TempDir(), nil
			},
			wantErr: true,
		},
		"trailing comma": {
			setup: func(t *testing.T) (string, []byte) {
				dir := t.TempDir()
				caCertPEM, _ := testutil.GenerateCA(t)
				writeFile(t, filepath.Join(dir, "ca.crt"), caCertPEM)
				return dir + ",", caCertPEM
			},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			caDirs, caCertPEM := tt.setup(t)

			pool, err := kedatls.BuildCAPool(caDirs)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if caCertPEM == nil {
				return
			}

			ca := parseCert(t, caCertPEM)
			if _, err := ca.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
				t.Fatalf("expected CA certificate to be trusted: %v", err)
			}
		})
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("decoding certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}
	return cert
}
