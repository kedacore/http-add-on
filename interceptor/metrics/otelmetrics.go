package metrics

import (
	"context"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/build"
)

type OtelMetrics struct {
	meter               api.Meter
	requestCounter      api.Int64Counter
	pendingRequestGauge api.Int64ObservableGauge
}

func NewOtelMetrics(metricsConfig *config.Metrics, options ...metric.Option) *OtelMetrics {
	ctx := context.Background()
	var exporter *otlpmetrichttp.Exporter
	var err error
	endpoint := otlpmetrichttp.WithEndpoint(metricsConfig.OtelHTTPCollectorEndpoint)
	headersFromEnvVar := getHeaders(metricsConfig.OtelHTTPHeaders)
	headers := otlpmetrichttp.WithHeaders(headersFromEnvVar)

	if metricsConfig.OtelHTTPCollectorInsecure {
		insecure := otlpmetrichttp.WithInsecure()
		exporter, err = otlpmetrichttp.New(ctx, endpoint, headers, insecure)
	} else {
		exporter, err = otlpmetrichttp.New(ctx, endpoint, headers)
	}

	if err != nil {
		log.Fatalf("could not create otelmetrichttp exporter: %v", err)
	}

	if options == nil {
		res := resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("interceptor-proxy"),
			semconv.ServiceVersionKey.String(build.Version()),
		)

		options = []metric.Option{
			metric.WithReader(metric.NewPeriodicReader(exporter, metric.WithInterval(time.Duration(metricsConfig.OtelMetricExportInterval)*time.Second))),
			metric.WithResource(res),
		}
	}

	provider := metric.NewMeterProvider(options...)
	meter := provider.Meter(meterName)

	reqCounter, err := meter.Int64Counter("interceptor_request_count", api.WithDescription("a counter of requests processed by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new otelhttpmetric request counter: %v", err)
	}

	pendingRequestGauge, err := meter.Int64ObservableGauge("interceptor_pending_request_count", api.WithDescription("a gauge of requests pending forwarding by the interceptor proxy"))
	if err != nil {
		log.Fatalf("could not create new otelhttpmetric pending request gauge: %v", err)
	}

	return &OtelMetrics{
		meter:               meter,
		requestCounter:      reqCounter,
		pendingRequestGauge: pendingRequestGauge,
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
	opt := api.WithAttributeSet(
		attribute.NewSet(
			attribute.Key("host").String(host),
		),
	)

	_, err := om.meter.RegisterCallback(func(_ context.Context, o api.Observer) error {
		o.ObserveInt64(om.pendingRequestGauge, value, opt)
		return nil
	}, om.pendingRequestGauge)
	if err != nil {
		log.Printf("error recording pending request values in prometheus gauge: %v", err)
	}
}

func getHeaders(s string) map[string]string {
	// Get the headers in key-value pair from the KEDA_HTTP_OTEL_HTTP_HEADERS environment variable
	var m = map[string]string{}

	if s != "" {
		h := strings.Split(s, ",")
		for _, v := range h {
			x := strings.Split(v, "=")
			m[x[0]] = x[1]
		}
	}

	return m
}
