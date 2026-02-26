package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kedacore/http-add-on/interceptor/config"
)

func TestTracingConfig(t *testing.T) {
	tracingCfg := config.MustParseTracing()
	tracingCfg.Enabled = true

	// check defaults are set correctly
	assert.Equal(t, "console", tracingCfg.Exporter)
	assert.True(t, tracingCfg.Enabled)
}
