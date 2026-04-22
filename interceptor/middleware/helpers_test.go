package middleware

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func requireMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return &m
			}
		}
	}

	t.Fatalf("metric not found: %s", name)

	return nil
}
