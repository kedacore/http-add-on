package tls

import (
	"testing"
)

// FuzzParseTLSVersion exercises the TLS version parser with arbitrary strings.
// The function parses user-supplied configuration, so it must never panic.
func FuzzParseTLSVersion(f *testing.F) {
	f.Add("1.2")
	f.Add("1.3")
	f.Add("TLS12")
	f.Add("TLS13")
	f.Add("tls12")
	f.Add("tls13")
	f.Add("")
	f.Add("1.0")
	f.Add("1.1")
	f.Add("TLSv1.2")
	f.Add("ssl3")

	f.Fuzz(func(t *testing.T, v string) {
		_, _ = parseTLSVersion(v)
	})
}

// FuzzParseCipherSuites exercises cipher-suite name parsing with arbitrary
// strings. The function parses user-supplied configuration, so it must never
// panic.
func FuzzParseCipherSuites(f *testing.F) {
	f.Add("TLS_AES_128_GCM_SHA256")
	f.Add("TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384")
	f.Add("")
	f.Add(",,,")
	f.Add("UNKNOWN_SUITE")
	f.Add("TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseCipherSuites(s)
	})
}

// FuzzParseCurvePreferences exercises curve preference parsing with arbitrary
// strings. The function parses user-supplied configuration, so it must never
// panic.
func FuzzParseCurvePreferences(f *testing.F) {
	f.Add("CurveP256")
	f.Add("X25519")
	f.Add("P-256,P-384")
	f.Add("")
	f.Add(",,,")
	f.Add("UNKNOWN_CURVE")
	f.Add("CurveP256, X25519, CurveP384")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseCurvePreferences(s)
	})
}
