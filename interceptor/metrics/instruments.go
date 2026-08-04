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
	ServiceName = "keda-http-interceptor"

	MetricRequestConcurrency = "interceptor.request.concurrency"
	MetricRequestCount       = "interceptor.request.count"
	MetricRequestDuration    = "interceptor.request.duration"
	MetricRequestRetries     = "interceptor.request.retries"

	AttrCode           = "code"
	AttrMethod         = "method"
	AttrOutcome        = "outcome"
	AttrRouteName      = "route_name"
	AttrRouteNamespace = "route_namespace"

	// OutcomeSuccess / OutcomeFailure are the values of AttrOutcome for
	// MetricRequestRetries: whether the retried request eventually got an
	// upstream response.
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"

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
	retryCounter    api.Int64Counter
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
		MetricRequestCount,
		api.WithDescription("Total requests processed by the interceptor proxy"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram(
		MetricRequestDuration,
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
		MetricRequestConcurrency,
		api.WithDescription("Concurrent requests at the interceptor proxy"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pending requests counter: %w", err)
	}

	retryCounter, err := meter.Int64Counter(
		MetricRequestRetries,
		api.WithDescription("Requests retried against an alternate upstream endpoint after a connect failure"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating retry counter: %w", err)
	}

	return &Instruments{
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
		pendingRequests: pendingRequests,
		retryCounter:    retryCounter,
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

// RecordRetry records a request that was retried against an alternate upstream
// endpoint, with success reporting whether the retried request eventually got
// an upstream response.
func (i *Instruments) RecordRetry(routeName, routeNamespace string, success bool) {
	outcome := OutcomeSuccess
	if !success {
		outcome = OutcomeFailure
	}
	attrs := api.WithAttributeSet(attribute.NewSet(
		attribute.String(AttrOutcome, outcome),
		attribute.String(AttrRouteName, routeName),
		attribute.String(AttrRouteNamespace, routeNamespace),
	))
	i.retryCounter.Add(context.Background(), 1, attrs)
}
