package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	meterName   = "keda-interceptor-proxy"
	serviceName = "interceptor-proxy"

	MetricPendingRequests        = "interceptor_pending_requests"
	MetricRequestDurationSeconds = "interceptor_request_duration_seconds"
	MetricRequests               = "interceptor_requests"

	AttrCode           = "code"
	AttrMethod         = "method"
	AttrRouteName      = "route_name"
	AttrRouteNamespace = "route_namespace"

	// MethodOther is the normalized value for non-standard HTTP methods,
	// following the OTel semantic convention prefix for synthetic values.
	MethodOther = "_OTHER"
)

// standardMethods is the set of HTTP methods that pass through normalization.
// Non-standard methods are mapped to MethodOther to bound cardinality.
var standardMethods = map[string]bool{
	http.MethodConnect: true,
	http.MethodDelete:  true,
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodPatch:   true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodTrace:   true,
}

// Instruments holds all metric instruments for the interceptor.
type Instruments struct {
	pendingRequests api.Int64UpDownCounter
	requestCounter  api.Int64Counter
	requestDuration api.Float64Histogram
}

// NewNoopInstruments returns Instruments backed by a no-op provider, for use in tests.
func NewNoopInstruments() *Instruments {
	i, err := NewInstruments(sdkmetric.NewMeterProvider())
	if err != nil {
		panic("creating noop instruments: " + err.Error())
	}
	return i
}

// NewInstruments creates metric instruments from a MeterProvider.
func NewInstruments(provider *sdkmetric.MeterProvider) (*Instruments, error) {
	meter := provider.Meter(meterName)

	requestCounter, err := meter.Int64Counter(
		MetricRequests,
		api.WithDescription("Total requests processed by the interceptor proxy"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram(
		MetricRequestDurationSeconds,
		api.WithDescription("Time from request received to response written"),
		api.WithUnit("s"),
		// Bucket boundaries from OTel HTTP semconv: https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
		api.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request duration histogram: %w", err)
	}

	pendingRequests, err := meter.Int64UpDownCounter(
		MetricPendingRequests,
		api.WithDescription("Requests currently in-flight at the interceptor proxy"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pending requests counter: %w", err)
	}

	return &Instruments{
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
		pendingRequests: pendingRequests,
	}, nil
}

func normalizeMethod(method string) string {
	if standardMethods[method] {
		return method
	}
	return MethodOther
}

// RecordRequest records a completed request with bounded labels.
func (i *Instruments) RecordRequest(method string, code int, routeName, routeNamespace string, duration time.Duration) {
	attrs := api.WithAttributeSet(attribute.NewSet(
		attribute.Int(AttrCode, code),
		attribute.String(AttrMethod, normalizeMethod(method)),
		attribute.String(AttrRouteName, routeName),
		attribute.String(AttrRouteNamespace, routeNamespace),
	))
	i.requestCounter.Add(context.Background(), 1, attrs)
	i.requestDuration.Record(context.Background(), duration.Seconds(), attrs)
}

// RecordPendingRequest increments or decrements the pending request gauge.
func (i *Instruments) RecordPendingRequest(routeName, routeNamespace string, delta int64) {
	attrs := api.WithAttributeSet(attribute.NewSet(
		attribute.String(AttrRouteName, routeName),
		attribute.String(AttrRouteNamespace, routeNamespace),
	))
	i.pendingRequests.Add(context.Background(), delta, attrs)
}
