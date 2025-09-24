package metrics

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kedacore/http-add-on/pkg/build"
)

var otLog = logf.Log.WithName("otel_collector")

type OtelMetrics struct {
	meter                   api.Meter
	httpScaledObjectCounter api.Int64UpDownCounter
}

func NewOtelMetrics(options ...metric.Option) *OtelMetrics {
	if options == nil {
		protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")

		var exporter metric.Exporter
		var err error
		switch protocol {
		case "grpc":
			otLog.V(1).Info("start OTEL grpc client")
			exporter, err = otlpmetricgrpc.New(context.Background())
		default:
			otLog.V(1).Info("start OTEL http client")
			exporter, err = otlpmetrichttp.New(context.Background())
		}

		if err != nil {
			log.Fatalf("could not create otelmetrichttp exporter: %v", err)
			return nil
		}
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
