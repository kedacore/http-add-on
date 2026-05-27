package tracing

import (
	"context"

	"go.opentelemetry.io/otel/propagation"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/observability"
)

const serviceName = "keda-http-interceptor"

func SetupOTelSDK(ctx context.Context, tCfg config.Tracing) (shutdown func(context.Context) error, err error) {
	return observability.SetupTracing(ctx, serviceName, tCfg)
}

func NewPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
