package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregate(t *testing.T) {
	r := require.New(t)
	counts := NewCounts()
	counts.Counts = map[string]Count{
		"host1": {
			Concurrency: 123,
			RPS:         123,
		},
		"host2": {
			Concurrency: 234,
			RPS:         234,
		},
		"host3": {
			Concurrency: 345,
			RPS:         345,
		},
		"host4": {
			Concurrency: 456,
			RPS:         456,
		},
	}
	expectedConcurrency := 0
	expectedRPS := 0.
	for _, v := range counts.Counts {
		expectedConcurrency += v.Concurrency
		expectedRPS += v.RPS
	}
	agg := counts.Aggregate()
	r.Equal(expectedConcurrency, agg.Concurrency)
	r.InDelta(expectedRPS, agg.RPS, 0)
}
