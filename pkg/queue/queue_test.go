package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrent(t *testing.T) {
	r := require.New(t)
	memory := NewMemory()
	err := memory.Resize("host1", 1)
	r.NoError(err)
	current, err := memory.Current()
	r.NoError(err)
	r.Equal(current.Counts, memory.countMap)

	err = memory.Resize("host1", 1)
	r.NoError(err)
	err = memory.Resize("host2", 1)
	r.NoError(err)
	r.NotEqual(current.Counts, memory.countMap)
}
