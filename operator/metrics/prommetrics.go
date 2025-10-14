package metrics

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/kedacore/http-add-on/pkg/build"
)

type PrometheusMetrics struct {
	meter                   api.Meter
	httpScaledObjectCounter api.Int64UpDownCounter
}

func NewPrometheusMetrics(options ...prometheus.Option) *PrometheusMetrics {
	var exporter *prometheus.Exporter
	var err error
	if options == nil {
		exporter, err = prometheus.New()
	} else {
		exporter, err = prometheus.New(options...)
	}
	if err != nil {
		log.Fatalf("could not create Prometheus exporter: %v", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("http-add-on-operator"),
		semconv.ServiceVersionKey.String(build.Version()),
	)

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)
	meter := provider.Meter(meterName)

	httpScaledObjectCounter, err := meter.Int64UpDownCounter("keda_http_scaled_object_total", api.WithDescription("a counter of http_scaled_objects processed by the operator"))
	if err != nil {
		log.Fatalf("could not create new Prometheus request counter: %v", err)
	}

	return &PrometheusMetrics{
		meter:                   meter,
		httpScaledObjectCounter: httpScaledObjectCounter,
	}
}

func (p *PrometheusMetrics) RecordHTTPScaledObjectCount(namespace string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("namespace").String(namespace),
		),
	)
	p.httpScaledObjectCounter.Add(ctx, 1, opt)
}

func (p *PrometheusMetrics) RecordDeleteHTTPScaledObjectCount(namespace string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("namespace").String(namespace),
		),
	)
	p.httpScaledObjectCounter.Add(ctx, -1, opt)
}
