//go:build e2e

package tls_test

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"

	h "github.com/kedacore/http-add-on/test/helpers"
)

var testenv env.Environment

const (
	tlsPort   = 8443
	tlsDomain = "example.com"
)

func TestMain(m *testing.M) {
	testenv = h.NewTestEnv(h.WithProxyPort(tlsPort), h.WithRequiredNamespaces(h.CertManagerNamespace))

	h.SetupCAHierarchy(testenv)

	// TODO: Use proper CA trust configuration instead of InsecureSkipVerify once
	// the interceptor supports a dedicated CA trust mechanism (see interceptor/tls_config.go).
	h.PatchInterceptorDeployment(testenv,
		h.WithTLSCert([]string{"*." + tlsDomain}),
		h.WithEnvVar("KEDA_HTTP_PROXY_TLS_ENABLED", "true"),
		h.WithEnvVar("KEDA_HTTP_PROXY_TLS_SKIP_VERIFY", "true"),
	)

	os.Exit(testenv.Run(m))
}
