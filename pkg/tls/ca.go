// Package tls provides CA pool and TLS policy helpers.
package tls

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildCAPool returns the system CA pool plus any PEM certificates found in
// the comma-separated caDirs directories.
func BuildCAPool(caDirs string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("loading system CA pool: %w", err)
	}
	if err := appendCADirs(pool, caDirs); err != nil {
		return nil, err
	}
	return pool, nil
}

// appendCADirs loads PEM CA certs from each comma-separated directory in
// caDirs into pool.
func appendCADirs(pool *x509.CertPool, caDirs string) error {
	var caLoaded bool

	for dir := range strings.SplitSeq(caDirs, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("reading CA directory %q: %w", dir, err)
		}

		for _, entry := range entries {
			// Skip subdirs and Kubernetes projected-volume "..data" metadata.
			if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path) //nolint:gosec // G304: path from configured CA directory
			if err != nil {
				return fmt.Errorf("reading CA certificate file %q: %w", path, err)
			}

			if !pool.AppendCertsFromPEM(data) {
				// Be strict for now and fail on any error, accept only valid files
				return fmt.Errorf("no PEM certificates found in %s", path)
			}
			caLoaded = true
		}
	}

	if caDirs != "" && !caLoaded {
		return fmt.Errorf("KEDA_HTTP_TLS_CA_DIRS is configured but no CA certificates were loaded from any directory")
	}
	return nil
}
