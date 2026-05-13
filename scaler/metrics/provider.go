package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/kedacore/http-add-on/pkg/build"
)

// NewMeterProvider creates a MeterProvider with Prometheus and/or OTLP readers.
// Without readers, all instrument operations become no-ops.
func NewMeterProvider(promEnabled, otlpEnabled bool, opts ...sdkmetric.Option) (*sdkmetric.MeterProvider, error) {
	var options []sdkmetric.Option

	if promEnabled {
		promExporter, err := prometheus.New(
			prometheus.WithoutScopeInfo(),
		)
		if err != nil {
			return nil, fmt.Errorf("creating prometheus exporter: %w", err)
		}
		options = append(options, sdkmetric.WithReader(promExporter))
	}

	if otlpEnabled {
		otlpExporter, err := otlpmetrichttp.New(context.Background())
		if err != nil {
			return nil, fmt.Errorf("creating OTLP exporter: %w", err)
		}
		options = append(options, sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(otlpExporter),
		))
	}

	options = append(options, sdkmetric.WithResource(
		resource.NewSchemaless(
			attribute.String("service.name", ServiceName),
			attribute.String("service.version", build.Version()),
		),
	))

	options = append(options, opts...)

	return sdkmetric.NewMeterProvider(options...), nil
}
