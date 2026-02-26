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
)

// TLSOptions holds TLS configuration options for the proxy server.
type TLSOptions struct {
	CertificatePath    string
	KeyPath            string
	CertStorePaths     string
	InsecureSkipVerify bool
}

// BuildTLSConfig creates a tls.Config from the given TLS options.
// The matching between request and certificate is performed by comparing TLS/SNI server name with x509 SANs.
func BuildTLSConfig(opts TLSOptions, logger logr.Logger) (*tls.Config, error) {
	servingTLS := &tls.Config{
		RootCAs:            defaultCertPool(logger),
		InsecureSkipVerify: opts.InsecureSkipVerify, //nolint:gosec // G402: user-configurable
	}
	var defaultCert *tls.Certificate

	uriDomainsToCerts := make(map[string]tls.Certificate)
	if opts.CertificatePath != "" && opts.KeyPath != "" {
		cert, err := addCert(uriDomainsToCerts, opts.CertificatePath, opts.KeyPath, logger)
		if err != nil {
			return nil, fmt.Errorf("error adding certificate and key: %w", err)
		}
		defaultCert = cert
		rawCert, err := os.ReadFile(opts.CertificatePath)
		if err != nil {
			return nil, fmt.Errorf("error reading certificate: %w", err)
		}
		servingTLS.RootCAs.AppendCertsFromPEM(rawCert)
	}

	if opts.CertStorePaths != "" {
		if err := loadCertStorePaths(opts.CertStorePaths, uriDomainsToCerts, servingTLS.RootCAs, logger); err != nil {
			return nil, err
		}
	}

	servingTLS.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cert, ok := uriDomainsToCerts[hello.ServerName]; ok {
			return &cert, nil
		}
		if defaultCert != nil {
			return defaultCert, nil
		}
		return nil, fmt.Errorf("no certificate found for %s", hello.ServerName)
	}
	servingTLS.Certificates = slices.Collect(maps.Values(uriDomainsToCerts))
	return servingTLS, nil
}

// loadCertStorePaths loads certificates from comma-separated directory paths.
func loadCertStorePaths(certStorePaths string, certs map[string]tls.Certificate, rootCAs *x509.CertPool, logger logr.Logger) error {
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
		if _, err := addCert(certs, certPath, keyPath, logger); err != nil {
			return fmt.Errorf("error adding certificate %s: %w", certPath, err)
		}
		rawCert, err := os.ReadFile(certPath) //nolint:gosec // G304: path from configured cert directory
		if err != nil {
			return fmt.Errorf("error reading certificate: %w", err)
		}
		rootCAs.AppendCertsFromPEM(rawCert)
	}

	return nil
}

// addCert adds a certificate to the map of certificates based on the certificate's SANs.
func addCert(m map[string]tls.Certificate, certPath, keyPath string, logger logr.Logger) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("error loading certificate and key: %w", err)
	}
	if cert.Leaf == nil {
		if len(cert.Certificate) == 0 {
			return nil, fmt.Errorf("no certificate found in certificate chain")
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("error parsing certificate: %w", err)
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
	return &cert, nil
}

// defaultCertPool returns the system cert pool or an empty pool if unavailable.
func defaultCertPool(logger logr.Logger) *x509.CertPool {
	systemCAs, err := x509.SystemCertPool()
	if err == nil {
		return systemCAs
	}

	logger.Error(err, "error loading system CA pool, using empty pool")
	return x509.NewCertPool()
}
