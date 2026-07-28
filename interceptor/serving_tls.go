package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"

	kedatls "github.com/kedacore/http-add-on/pkg/tls"
)

// ServingTLSOptions holds certificate configuration for the proxy server's
// listening side.
type ServingTLSOptions struct {
	CertificatePath string
	KeyPath         string
	CertStorePaths  string
}

// BuildServingTLSConfig creates a tls.Config from the given TLS options.
// The matching between request and certificate is performed by comparing TLS/SNI server name with x509 SANs.
// When CertificatePath and KeyPath are set, a certwatcher is created for hot-reload of the default cert.
// The caller must start the returned watcher with watcher.Start(ctx).
func BuildServingTLSConfig(opts ServingTLSOptions, policy kedatls.Policy, logger logr.Logger) (*tls.Config, *certwatcher.CertWatcher, error) {
	servingTLS, err := policy.NewConfig()
	if err != nil {
		return nil, nil, err
	}

	var watcher *certwatcher.CertWatcher

	uriDomainsToCerts := make(map[string]tls.Certificate)
	if opts.CertificatePath != "" && opts.KeyPath != "" {
		var err error
		watcher, err = certwatcher.New(opts.CertificatePath, opts.KeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("creating cert watcher: %w", err)
		}
	}

	if opts.CertStorePaths != "" {
		if err := loadCertStorePaths(opts.CertStorePaths, uriDomainsToCerts, logger); err != nil {
			return nil, nil, err
		}
	}

	// TODO: uriDomainsToCerts is a snapshot from startup — CertStorePaths certs
	// are not hot-reloaded. Only the default cert (via certwatcher) supports
	// hot-reload. Add directory-level watching or similar to reload all certs.
	servingTLS.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// Exact SNI match from the static cert map takes priority
		if cert, ok := uriDomainsToCerts[hello.ServerName]; ok {
			return &cert, nil
		}
		// Fall back to certwatcher-managed default cert (hot-reloaded)
		if watcher != nil {
			return watcher.GetCertificate(hello)
		}
		return nil, fmt.Errorf("no certificate found for %s", hello.ServerName)
	}
	servingTLS.Certificates = slices.Collect(maps.Values(uriDomainsToCerts))
	return servingTLS, watcher, nil
}

// loadCertStorePaths loads certificates from comma-separated directory paths.
func loadCertStorePaths(certStorePaths string, certs map[string]tls.Certificate, logger logr.Logger) error {
	certFiles := make(map[string]string)
	keyFiles := make(map[string]string)
	dirPaths := strings.SplitSeq(certStorePaths, ",")

	for dir := range dirPaths {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			switch {
			case strings.HasSuffix(path, "-key.pem"):
				certID := path[:len(path)-8]
				keyFiles[certID] = path
			case strings.HasSuffix(path, ".pem"):
				certID := path[:len(path)-4]
				certFiles[certID] = path
			case strings.HasSuffix(path, ".key"):
				certID := path[:len(path)-4]
				keyFiles[certID] = path
			case strings.HasSuffix(path, ".crt"):
				certID := path[:len(path)-4]
				certFiles[certID] = path
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error walking certificate store: %w", err)
		}
	}

	for certID, certPath := range certFiles {
		logger.Info("adding certificate", "certID", certID, "certPath", certPath)
		keyPath, ok := keyFiles[certID]
		if !ok {
			return fmt.Errorf("no key found for certificate %s", certPath)
		}
		if err := addCert(certs, certPath, keyPath, logger); err != nil {
			return fmt.Errorf("error adding certificate %s: %w", certPath, err)
		}
	}

	return nil
}

// addCert adds a certificate to the map of certificates based on the certificate's SANs.
func addCert(m map[string]tls.Certificate, certPath, keyPath string, logger logr.Logger) error {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("error loading certificate and key: %w", err)
	}
	if cert.Leaf == nil {
		if len(cert.Certificate) == 0 {
			return fmt.Errorf("no certificate found in certificate chain")
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("error parsing certificate: %w", err)
		}
	}
	for _, d := range cert.Leaf.DNSNames {
		logger.Info("adding certificate", "dns", d)
		m[d] = cert
	}
	for _, ip := range cert.Leaf.IPAddresses {
		logger.Info("adding certificate", "ip", ip.String())
		m[ip.String()] = cert
	}
	for _, uri := range cert.Leaf.URIs {
		logger.Info("adding certificate", "uri", uri.String())
		m[uri.String()] = cert
	}
	return nil
}
