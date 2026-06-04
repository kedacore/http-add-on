package metrics

import (
	"context"
	"fmt"
	"time"

	api "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	meterName = "keda-external-scaler"

	// ServiceName is the OTEL service.name used for both metrics and tracing.
	ServiceName = "keda-http-external-scaler"

	MetricPingerFetchDuration   = "scaler.pinger.fetch.duration"
	MetricPingerFetchErrors     = "scaler.pinger.fetch.errors"
	MetricPingerUnreachablePods = "scaler.pinger.unreachable_pods"
	MetricPingerEndpoints       = "scaler.pinger.endpoints"
)

// Instruments holds all metric instruments for the external scaler.
type Instruments struct {
	pingerFetchDuration   api.Float64Histogram
	pingerFetchErrors     api.Int64Counter
	pingerUnreachablePods api.Int64Gauge
	pingerEndpoints       api.Int64Gauge
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

	pingerFetchDuration, err := meter.Float64Histogram(
		MetricPingerFetchDuration,
		api.WithDescription("Duration of a queue pinger fetch cycle across all interceptor pods"),
		api.WithUnit("s"),
		api.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pinger fetch duration histogram: %w", err)
	}

	pingerFetchErrors, err := meter.Int64Counter(
		MetricPingerFetchErrors,
		api.WithDescription("Total failed queue pinger fetch cycles"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pinger fetch errors counter: %w", err)
	}

	pingerUnreachablePods, err := meter.Int64Gauge(
		MetricPingerUnreachablePods,
		api.WithDescription("Number of interceptor pods that were unreachable in the last fetch cycle"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pinger unreachable pods gauge: %w", err)
	}

	pingerEndpoints, err := meter.Int64Gauge(
		MetricPingerEndpoints,
		api.WithDescription("Number of interceptor endpoints the scaler is polling"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pinger endpoints gauge: %w", err)
	}

	return &Instruments{
		pingerFetchDuration:   pingerFetchDuration,
		pingerFetchErrors:     pingerFetchErrors,
		pingerUnreachablePods: pingerUnreachablePods,
		pingerEndpoints:       pingerEndpoints,
	}, nil
}

// RecordFetch records a completed pinger fetch cycle.
func (i *Instruments) RecordFetch(duration time.Duration, endpointCount, failedPods int, fetchErr error) {
	i.pingerFetchDuration.Record(context.Background(), duration.Seconds())
	i.pingerEndpoints.Record(context.Background(), int64(endpointCount))
	i.pingerUnreachablePods.Record(context.Background(), int64(failedPods))
	if fetchErr != nil {
		i.pingerFetchErrors.Add(context.Background(), 1)
	}
}
