package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kedacore/http-add-on/interceptor/config"
)

func TestTracingConfig(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "console")

	tracingCfg := config.MustParseTracing()
	tracingCfg.Enabled = true

	assert.Equal(t, "console", tracingCfg.Exporter)
	assert.True(t, tracingCfg.Enabled)
}
