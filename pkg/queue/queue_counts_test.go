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
			Concurrency:  123,
			RequestCount: 1000,
			RPS:          123,
		},
		"host2": {
			Concurrency:  234,
			RequestCount: 2000,
			RPS:          234,
		},
		"host3": {
			Concurrency:  345,
			RequestCount: 3000,
			RPS:          345,
		},
		"host4": {
			Concurrency:  456,
			RequestCount: 4000,
			RPS:          456,
		},
	}
	expectedConcurrency := 0
	expectedRequestCount := int64(0)
	expectedRPS := 0.
	for _, v := range counts.Counts {
		expectedConcurrency += v.Concurrency
		expectedRequestCount += v.RequestCount
		expectedRPS += v.RPS
	}
	agg := counts.Aggregate()
	r.Equal(expectedConcurrency, agg.Concurrency)
	r.Equal(expectedRequestCount, agg.RequestCount)
	r.InDelta(expectedRPS, agg.RPS, 0)
}
