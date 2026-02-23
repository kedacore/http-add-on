package metrics

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/kedacore/http-add-on/pkg/build"
)

type PrometheusMetrics struct {
	meter                 api.Meter
	requestCounter        api.Int64Counter
	pendingRequestCounter api.Int64UpDownCounter
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
		semconv.ServiceName("interceptor-proxy"),
		semconv.ServiceVersion(build.Version()),
	)

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)
	meter := provider.Meter(meterName)

	reqCounter, err := meter.Int64Counter("interceptor_request_count", api.WithDescription("a counter of requests processed by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new Prometheus request counter: %v", err)
	}

	pendingRequestCounter, err := meter.Int64UpDownCounter("interceptor_pending_request_count", api.WithDescription("a count of requests pending forwarding by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new Prometheus pending request counter: %v", err)
	}

	return &PrometheusMetrics{
		meter:                 meter,
		requestCounter:        reqCounter,
		pendingRequestCounter: pendingRequestCounter,
	}
}

func (p *PrometheusMetrics) RecordRequestCount(method string, path string, responseCode int, host string) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("method").String(method),
			attribute.Key("path").String(path),
			attribute.Key("code").Int(responseCode),
			attribute.Key("host").String(host),
		),
	)
	p.requestCounter.Add(ctx, 1, opt)
}

func (p *PrometheusMetrics) RecordPendingRequestCount(host string, value int64) {
	ctx := context.Background()
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("host").String(host),
		),
	)

	p.pendingRequestCounter.Add(ctx, value, opt)
}
