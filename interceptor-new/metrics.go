package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	ctrl "sigs.k8s.io/controller-runtime"
)

var metricsLog = ctrl.Log.WithName("metrics")

// MetricsCollector holds Prometheus and optional OTel metrics.
type MetricsCollector struct {
	// Prometheus
	requestCount        *prometheus.CounterVec
	pendingRequestCount *prometheus.GaugeVec
	registry            *prometheus.Registry

	// OTel (nil when disabled)
	otelRequestCounter api.Int64Counter
	otelPendingCounter api.Int64UpDownCounter
	otelShutdown       func(context.Context) error
}

// NewMetricsCollector creates and registers Prometheus metrics.
// If otelMetricsEnabled is true, an OTLP HTTP metric exporter is also started.
func NewMetricsCollector(otelMetricsEnabled bool) *MetricsCollector {
	registry := prometheus.NewRegistry()

	requestCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "interceptor_request_count_total",
			Help: "A counter of requests processed by the interceptor proxy",
		},
		[]string{"method", "path", "code", "host"},
	)
	registry.MustRegister(requestCount)

	pendingRequestCount := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "interceptor_pending_request_count",
			Help: "A count of requests pending forwarding by the interceptor proxy",
		},
		[]string{"host"},
	)
	registry.MustRegister(pendingRequestCount)

	mc := &MetricsCollector{
		requestCount:        requestCount,
		pendingRequestCount: pendingRequestCount,
		registry:            registry,
	}

	if otelMetricsEnabled {
		mc.initOtelMetrics()
	}

	return mc
}

// initOtelMetrics sets up the OTLP HTTP metric exporter and registers
// the two OTel instruments that mirror the Prometheus counters.
func (m *MetricsCollector) initOtelMetrics() {
	ctx := context.Background()

	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		metricsLog.Error(err, "Failed to create OTel metrics exporter")
		return
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("interceptor-proxy"),
		))
	if err != nil {
		metricsLog.Error(err, "Failed to create OTel resource")
		_ = exporter.Shutdown(ctx)
		return
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(res),
	)
	meter := provider.Meter("keda-interceptor-proxy")

	reqCounter, err := meter.Int64Counter("interceptor_request_count",
		api.WithDescription("A counter of requests processed by the interceptor proxy"))
	if err != nil {
		metricsLog.Error(err, "Failed to create OTel request counter")
		_ = provider.Shutdown(ctx)
		return
	}

	pendingCounter, err := meter.Int64UpDownCounter("interceptor_pending_request_count",
		api.WithDescription("A count of requests pending forwarding by the interceptor proxy"))
	if err != nil {
		metricsLog.Error(err, "Failed to create OTel pending counter")
		_ = provider.Shutdown(ctx)
		return
	}

	m.otelRequestCounter = reqCounter
	m.otelPendingCounter = pendingCounter
	m.otelShutdown = provider.Shutdown

	metricsLog.Info("OTel metrics exporter enabled")
}

// Shutdown gracefully shuts down the OTel metrics exporter (if active).
func (m *MetricsCollector) Shutdown(ctx context.Context) {
	if m.otelShutdown != nil {
		_ = m.otelShutdown(ctx)
	}
}

// RecordRequest increments the completed request counter.
func (m *MetricsCollector) RecordRequest(method, path string, code int, host string) {
	m.requestCount.WithLabelValues(method, path, strconv.Itoa(code), host).Inc()

	if m.otelRequestCounter != nil {
		m.otelRequestCounter.Add(context.Background(), 1,
			api.WithAttributeSet(attribute.NewSet(
				attribute.String("method", method),
				attribute.String("path", path),
				attribute.Int("code", code),
				attribute.String("host", host),
			)))
	}
}

// RecordPending sets the in-flight request gauge for a host.
func (m *MetricsCollector) RecordPending(host string, value float64) {
	m.pendingRequestCount.WithLabelValues(host).Set(value)

	if m.otelPendingCounter != nil {
		m.otelPendingCounter.Add(context.Background(), int64(value),
			api.WithAttributeSet(attribute.NewSet(
				attribute.String("host", host),
			)))
	}
}

// MetricsServer serves Prometheus metrics on the metrics port.
type MetricsServer struct {
	config  *Config
	metrics *MetricsCollector
}

// NewMetricsServer creates a new metrics server.
func NewMetricsServer(config *Config, metrics *MetricsCollector) *MetricsServer {
	return &MetricsServer{
		config:  config,
		metrics: metrics,
	}
}

// ListenAndServe starts the metrics server.
func (ms *MetricsServer) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(ms.metrics.registry, promhttp.HandlerOpts{}))

	addr := fmt.Sprintf(":%d", ms.config.MetricsPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	metricsLog.Info("Metrics server listening", "addr", addr)
	return server.ListenAndServe()
}
