//go:build e2e

package observability_test

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"

	h "github.com/kedacore/http-add-on/test/helpers"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = h.NewTestEnv(h.WithRequiredNamespaces(otelNamespace, jaegerNamespace))

	h.PatchInterceptorDeployment(testenv,
		h.WithEnvVar("OTEL_EXPORTER_OTLP_ENDPOINT", "http://opentelemetry-collector.open-telemetry-system:4318"),
		h.WithEnvVar("OTEL_EXPORTER_OTLP_METRICS_ENABLED", "true"),
		h.WithEnvVar("OTEL_EXPORTER_OTLP_TRACES_ENABLED", "true"),
		h.WithEnvVar("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/protobuf"),
		h.WithEnvVar("OTEL_METRIC_EXPORT_INTERVAL", "1"),
	)

	os.Exit(testenv.Run(m))
}
