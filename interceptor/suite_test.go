package main

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestMain unsets the proxy environment variables before any test runs.
// The proxy handler transport uses http.ProxyFromEnvironment, which reads
// them only once per process, so t.Setenv inside a test is not reliable.
// Tests route fake hostnames to local servers, which a proxy would break.
func TestMain(m *testing.M) {
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		if err := os.Unsetenv(k); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func TestInterceptor(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Interceptor Suite")
}
