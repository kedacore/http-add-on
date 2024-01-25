package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCurrent(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	err := memory.Resize("host1", 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	counts := map[string]int{}
	activities := map[string]time.Time{}
	for key, val := range current.Counts {
		counts[key] = val.Requests
		activities[key] = val.Activity
	}
	r.Equal(counts, memory.countMap)
	r.Equal(activities, memory.activityMap)

	err = memory.Resize("host1", 1)
	r.NoError(err)
	err = memory.Resize("host2", 1)
	r.NoError(err)
	r.NotEqual(counts, memory.countMap)
	r.NotEqual(activities, memory.activityMap)
}
