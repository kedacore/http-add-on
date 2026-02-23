package metrics

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/kedacore/http-add-on/pkg/build"
)

type OtelMetrics struct {
	meter                 api.Meter
	requestCounter        api.Int64Counter
	pendingRequestCounter api.Int64UpDownCounter
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
			semconv.ServiceName("interceptor-proxy"),
			semconv.ServiceVersion(build.Version()),
		)

		options = []metric.Option{
			metric.WithReader(metric.NewPeriodicReader(exporter)),
			metric.WithResource(res),
		}
	}

	provider := metric.NewMeterProvider(options...)
	meter := provider.Meter(meterName)

	reqCounter, err := meter.Int64Counter("interceptor_request_count", api.WithDescription("a counter of requests processed by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new otelhttpmetric request counter: %v", err)
	}

	pendingRequestCounter, err := meter.Int64UpDownCounter("interceptor_pending_request_count", api.WithDescription("a count of requests pending forwarding by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new otelhttpmetric pending request counter: %v", err)
	}

	return &OtelMetrics{
		meter:                 meter,
		requestCounter:        reqCounter,
		pendingRequestCounter: pendingRequestCounter,
	}
}

func (om *OtelMetrics) RecordRequestCount(method string, path string, responseCode int, host string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("method").String(method),
			attribute.Key("path").String(path),
			attribute.Key("code").Int(responseCode),
			attribute.Key("host").String(host),
		),
	)
	om.requestCounter.Add(ctx, 1, opt)
}

func (om *OtelMetrics) RecordPendingRequestCount(host string, value int64) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("host").String(host),
		),
	)

	om.pendingRequestCounter.Add(ctx, value, opt)
}
