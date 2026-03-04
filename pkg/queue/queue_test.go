package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	hostName = "host1"
)

func TestCurrent(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host, time.Minute, time.Second)

	err := memory.Increase(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(1, current.Counts[host].Concurrency)
	r.Greater(current.Counts[host].RPS, 0.0)

	err = memory.Increase(host, 1)
	r.NoError(err)
	err = memory.Increase(host, 1)
	r.NoError(err)
	current2, err := memory.Current()
	r.NoError(err)
	r.Equal(3, current2.Counts[host].Concurrency)
}

func TestDecreaseClamp(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host, time.Minute, time.Second)

	err := memory.Decrease(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(0, current.Counts[host].Concurrency)
}

func TestRemoveKey(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host, time.Minute, time.Second)

	r.True(memory.RemoveKey(host))
	r.False(memory.RemoveKey(host))

	current, err := memory.Current()
	r.NoError(err)
	r.Empty(current.Counts)
}

func TestUpdateBuckets(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host, time.Minute, time.Second)
	memory.UpdateBuckets(host, 2*time.Minute, 2*time.Second)

	err := memory.Increase(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(1, current.Counts[host].Concurrency)
}
