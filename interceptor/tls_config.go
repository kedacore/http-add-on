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
	MinTLSVersion      string
	MaxTLSVersion      string
	CipherSuites       string
	CurvePreferences   string
}

// BuildTLSConfig creates a tls.Config from the given TLS options.
// The matching between request and certificate is performed by comparing TLS/SNI server name with x509 SANs.
func BuildTLSConfig(opts TLSOptions, logger logr.Logger) (*tls.Config, error) {
	servingTLS := &tls.Config{
		RootCAs:            defaultCertPool(logger),
		InsecureSkipVerify: opts.InsecureSkipVerify, //nolint:gosec // G402: user-configurable
	}

	if opts.MinTLSVersion != "" {
		v, err := parseTLSVersion(opts.MinTLSVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS min version %q: %w", opts.MinTLSVersion, err)
		}
		servingTLS.MinVersion = v
	}
	if opts.MaxTLSVersion != "" {
		v, err := parseTLSVersion(opts.MaxTLSVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS max version %q: %w", opts.MaxTLSVersion, err)
		}
		servingTLS.MaxVersion = v
	}
	if opts.CipherSuites != "" {
		suites, err := parseCipherSuites(opts.CipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS cipher suites: %w", err)
		}
		servingTLS.CipherSuites = suites
	}
	if opts.CurvePreferences != "" {
		curves, err := parseCurvePreferences(opts.CurvePreferences)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS curve preferences: %w", err)
		}
		servingTLS.CurvePreferences = curves
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

// TODO: loadCertStorePaths mixes serving certs with CA trust. A dedicated
// CA trust mechanism that only appends to RootCAs without requiring a key is needed.

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

// parseTLSVersion converts a version string ("1.2" or "1.3") to the
// corresponding crypto/tls constant.
func parseTLSVersion(v string) (uint16, error) {
	switch v {
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q: must be %q or %q", v, "1.2", "1.3")
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
