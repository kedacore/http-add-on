package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const (
	meterName = "keda-external-scaler"

	// ServiceName is the OTEL service.name used for both metrics and tracing.
	ServiceName = "keda-http-external-scaler"

	MetricPingerFetchDuration = "scaler.pinger.fetch.duration"
	MetricPingerFetchErrors   = "scaler.pinger.fetch.errors"
	MetricPingerEndpoints     = "scaler.pinger.endpoints"

	AttrNamespace = "namespace"
	AttrService   = "service"
)

// Instruments holds all metric instruments for the external scaler.
type Instruments struct {
	pingerFetchDuration api.Float64Histogram
	pingerFetchErrors   api.Int64Counter
	pingerEndpoints     api.Int64Gauge
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

	pingerEndpoints, err := meter.Int64Gauge(
		MetricPingerEndpoints,
		api.WithDescription("Number of interceptor endpoints the scaler is polling"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating pinger endpoints gauge: %w", err)
	}

	return &Instruments{
		pingerFetchDuration: pingerFetchDuration,
		pingerFetchErrors:   pingerFetchErrors,
		pingerEndpoints:     pingerEndpoints,
	}, nil
}

// RecordFetch records a completed pinger fetch cycle.
func (i *Instruments) RecordFetch(duration time.Duration, endpointCount int, fetchErr error) {
	attrs := api.WithAttributeSet(attribute.NewSet())
	i.pingerFetchDuration.Record(context.Background(), duration.Seconds(), attrs)
	i.pingerEndpoints.Record(context.Background(), int64(endpointCount), attrs)
	if fetchErr != nil {
		i.pingerFetchErrors.Add(context.Background(), 1, attrs)
	}
}
