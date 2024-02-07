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
	r.Equal(current.Counts[host].RPS, memory.rpsMap[host].WindowAverage(now))
	currentForHost, validCount := memory.CurrentForHost("host1")
	r.Equal(validCount, true)
	r.Equal(currentForHost, 1)

	err = memory.Increase(host, 1)
	r.NoError(err)
	err = memory.Increase(host, 1)
	r.NoError(err)
	r.NotEqual(current.Counts[host].Concurrency, memory.concurrentMap[host])
	r.NotEqual(current.Counts[host].RPS, memory.rpsMap[host].WindowAverage(now))
}
