package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kedacore/http-add-on/pkg/observability"
)

func TestTracingConfig(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "console")

	tracingCfg := observability.MustParseTracingConfig()
	tracingCfg.Enabled = true

	assert.Equal(t, "console", tracingCfg.Exporter)
	assert.True(t, tracingCfg.Enabled)
}
