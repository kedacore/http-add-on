package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

var tlsLog = ctrl.Log.WithName("tls")

// BuildTLSConfig creates a tls.Config from the interceptor configuration.
// Certificates are matched to incoming requests via TLS/SNI server name
// against x509 SANs, with the primary cert/key pair used as the default.
func BuildTLSConfig(cfg *Config) (*tls.Config, error) {
	rootCAs := defaultCertPool()

	tlsConfig := &tls.Config{
		RootCAs:            rootCAs,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.TLSSkipVerify, //nolint:gosec // user-configurable for dev/test
	}

	uriDomainsToCerts := make(map[string]tls.Certificate)
	var defaultCert *tls.Certificate

	// Load the primary certificate and key
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		cert, err := addCert(uriDomainsToCerts, cfg.TLSCertPath, cfg.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("loading primary TLS cert/key: %w", err)
		}
		defaultCert = cert

		rawCert, err := os.ReadFile(cfg.TLSCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading TLS certificate: %w", err)
		}
		rootCAs.AppendCertsFromPEM(rawCert)
	}

	// Load certificates from cert store directories
	if cfg.TLSCertStorePaths != "" {
		if err := loadCertStorePaths(cfg.TLSCertStorePaths, uriDomainsToCerts, rootCAs); err != nil {
			return nil, err
		}
	}

	// SNI-based certificate selection
	tlsConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cert, ok := uriDomainsToCerts[hello.ServerName]; ok {
			return &cert, nil
		}
		if defaultCert != nil {
			return defaultCert, nil
		}
		return nil, fmt.Errorf("no certificate found for %s", hello.ServerName)
	}

	certs := make([]tls.Certificate, 0, len(uriDomainsToCerts))
	for _, cert := range uriDomainsToCerts {
		certs = append(certs, cert)
	}
	tlsConfig.Certificates = certs

	return tlsConfig, nil
}

// loadCertStorePaths loads certificates from comma-separated directory paths.
func loadCertStorePaths(certStorePaths string, certs map[string]tls.Certificate, rootCAs *x509.CertPool) error {
	certFiles := make(map[string]string)
	keyFiles := make(map[string]string)

	for _, dir := range strings.Split(certStorePaths, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			switch {
			case strings.HasSuffix(path, "-key.pem"):
				certFiles[path[:len(path)-8]] = ""
				keyFiles[path[:len(path)-8]] = path
			case strings.HasSuffix(path, ".pem"):
				certFiles[path[:len(path)-4]] = path
			case strings.HasSuffix(path, ".key"):
				keyFiles[path[:len(path)-4]] = path
			case strings.HasSuffix(path, ".crt"):
				certFiles[path[:len(path)-4]] = path
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walking certificate store %s: %w", dir, err)
		}
	}

	for certID, certPath := range certFiles {
		if certPath == "" {
			continue
		}
		keyPath, ok := keyFiles[certID]
		if !ok {
			return fmt.Errorf("no key found for certificate %s", certPath)
		}
		tlsLog.Info("Loading TLS certificate from store", "cert", certPath)
		if _, err := addCert(certs, certPath, keyPath); err != nil {
			return fmt.Errorf("loading certificate %s: %w", certPath, err)
		}
		rawCert, err := os.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("reading certificate %s: %w", certPath, err)
		}
		rootCAs.AppendCertsFromPEM(rawCert)
	}

	return nil
}

// addCert loads a certificate+key pair and registers it by its SANs.
func addCert(m map[string]tls.Certificate, certPath, keyPath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("loading certificate and key: %w", err)
	}

	if cert.Leaf == nil {
		if len(cert.Certificate) == 0 {
			return nil, fmt.Errorf("no certificate found in chain")
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("parsing certificate: %w", err)
		}
	}

	for _, d := range cert.Leaf.DNSNames {
		tlsLog.V(1).Info("Registering TLS certificate", "dns", d)
		m[d] = cert
	}
	for _, ip := range cert.Leaf.IPAddresses {
		tlsLog.V(1).Info("Registering TLS certificate", "ip", ip.String())
		m[ip.String()] = cert
	}
	for _, uri := range cert.Leaf.URIs {
		tlsLog.V(1).Info("Registering TLS certificate", "uri", uri.String())
		m[uri.String()] = cert
	}

	return &cert, nil
}

// defaultCertPool returns the system cert pool or an empty pool if unavailable.
func defaultCertPool() *x509.CertPool {
	systemCAs, err := x509.SystemCertPool()
	if err == nil {
		return systemCAs
	}
	tlsLog.Info("Could not load system CA pool, using empty pool", "error", err)
	return x509.NewCertPool()
}
