package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregate(t *testing.T) {
	r := require.New(t)
	counts := NewCounts()
	counts.Counts = map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 456,
		"host4": 567,
	}
	expectedAgg := 0
	for _, v := range counts.Counts {
		expectedAgg += v
	}
	r.Equal(expectedAgg, counts.Aggregate())
}
