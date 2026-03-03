package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCurrent(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	now := time.Now()
	host := "host1"
	memory.EnsureKey(host, time.Minute, time.Second)
	err := memory.Increase(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(current.Counts[host].Concurrency, memory.concurrentMap[host])
	r.InDelta(current.Counts[host].RPS, memory.rpsMap[host].WindowAverage(now), 0)

	err = memory.Increase(host, 1)
	r.NoError(err)
	err = memory.Increase(host, 1)
	r.NoError(err)
	r.NotEqual(current.Counts[host].Concurrency, memory.concurrentMap[host])
	r.NotEqual(current.Counts[host].RPS, memory.rpsMap[host].WindowAverage(now))
}
