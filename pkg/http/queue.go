package http

import "sync"

// QueueCountReader represents the size of a virtual HTTP queue, possibly
// distributed across multiple HTTP server processes. It only can access
// the current size of the queue, not any other information about requests.
//
// It is concurrency safe.
type QueueCountReader interface {
	Current() (int, error)
}

// QueueCounter represents a virtual HTTP queue, possibly distributed across
// multiple HTTP server processes. It can only increase or decrease the
// size of the queue or read the current size of the queue, but not read
// or modify any other information about it.
//
// Both the mutation and read functionality is concurrency safe, but
// the read functionality is point-in-time only
type QueueCounter interface {
	QueueCountReader
	Resize(int) error
}

// MemoryQueue is a reference QueueCounter implementation that holds the
// HTTP queue in memory only. Always use NewMemoryQueue to create one
// of these.
type MemoryQueue struct {
	count   int
	mut *sync.RWMutex
}

// NewMemoryQueue creates a new empty memory queue
func NewMemoryQueue() *MemoryQueue {
	lock := new(sync.RWMutex)
	return &MemoryQueue{
		count: 0,
		mut: lock,
	}
}

// Resize changes the size of the queue. Further calls to Current() return
// the newly calculated size if no other Resize() calls were made in the
// interim.
func (r *MemoryQueue) Resize(delta int) error {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.count += delta
	return nil
}

// Current returns the current size of the queue.
func (r *MemoryQueue) Current() (int, error) {
	r.mut.RLock()
	defer r.mut.RUnlock()
	return r.count, nil
}
