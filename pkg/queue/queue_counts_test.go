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
			Requests: 123,
		},
		"host2": {
			Requests: 234,
		},
		"host3": {
			Requests: 345,
		},
		"host4": {
			Requests: 456,
		},
	}
	expectedAgg := 0
	for _, v := range counts.Counts {
		expectedAgg += v.Requests
	}
	r.Equal(expectedAgg, counts.Aggregate())
}
