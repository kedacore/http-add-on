package metrics

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/kedacore/http-add-on/pkg/build"
)

type OtelMetrics struct {
	meter                   api.Meter
	httpScaledObjectCounter api.Int64UpDownCounter
}

func NewOtelMetrics(options ...metric.Option) *OtelMetrics {
	ctx := context.Background()

	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		log.Fatalf("could not create otelmetrichttp exporter: %v", err)
	}

	if options == nil {
		res := resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("http-add-on-operator"),
			semconv.ServiceVersionKey.String(build.Version()),
		)

		options = []metric.Option{
			metric.WithReader(metric.NewPeriodicReader(exporter)),
			metric.WithResource(res),
		}
	}

	provider := metric.NewMeterProvider(options...)
	meter := provider.Meter(meterName)

	httpScaledObjectCounter, err := meter.Int64UpDownCounter("keda.http.scaled.object.total", api.WithDescription("a counter of http_scaled_objects processed by the operator"))
	if err != nil {
		log.Fatalf("could not create new otelhttpmetric request counter: %v", err)
	}

	return &OtelMetrics{
		meter:                   meter,
		httpScaledObjectCounter: httpScaledObjectCounter,
	}
}

func (om *OtelMetrics) RecordHTTPScaledObjectCount(namespace string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("namespace").String(namespace),
		),
	)
	om.httpScaledObjectCounter.Add(ctx, 1, opt)
}

func (om *OtelMetrics) RecordDeleteHTTPScaledObjectCount(namespace string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("namespace").String(namespace),
		),
	)
	om.httpScaledObjectCounter.Add(ctx, -1, opt)
}
