package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	hostName = "host1"
)

func TestCurrent(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host)

	err := memory.Increase(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(1, current[host].Concurrency)
	r.Equal(int64(1), current[host].RequestCount)

	err = memory.Increase(host, 1)
	r.NoError(err)
	err = memory.Increase(host, 1)
	r.NoError(err)
	current2, err := memory.Current()
	r.NoError(err)
	r.Equal(3, current2[host].Concurrency)
	r.Equal(int64(3), current2[host].RequestCount)
}

func TestDecreaseClamp(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host)

	err := memory.Decrease(host, 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(0, current[host].Concurrency)
}

func TestRemoveKey(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host)

	r.True(memory.RemoveKey(host))
	r.False(memory.RemoveKey(host))

	current, err := memory.Current()
	r.NoError(err)
	r.Empty(current)
}

func TestRequestCountMonotonic(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	host := hostName
	memory.EnsureKey(host)

	r.NoError(memory.Increase(host, 1))
	r.NoError(memory.Increase(host, 1))
	r.NoError(memory.Decrease(host, 1))

	current, err := memory.Current()
	r.NoError(err)
	r.Equal(1, current[host].Concurrency)
	r.Equal(int64(2), current[host].RequestCount,
		"RequestCount should keep growing even after Decrease")
}
