package routing

import (
	"sync/atomic"
)

type AtomicValue[V any] struct {
	atomicValue atomic.Value
}

func NewAtomicValue[V any](v V) *AtomicValue[V] {
	var av AtomicValue[V]
	av.Set(v)

	return &av
}

func (av *AtomicValue[V]) Get() V {
	if v, ok := av.atomicValue.Load().(V); ok {
		return v
	}

	return *new(V)
}

func (av *AtomicValue[V]) Set(v V) {
	av.atomicValue.Store(v)
}
