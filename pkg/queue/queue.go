package queue

import (
	"sync"
)

// CountReader represents the size of a virtual HTTP queue, possibly
// distributed across multiple HTTP server processes. It only can access
// the current size of the queue, not any other information about requests.
//
// It is concurrency safe.
type CountReader interface {
	// Current returns the current count of pending requests
	// for the given hostname
	Current() (*Counts, error)
}

// QueueCounter represents a virtual HTTP queue, possibly distributed across
// multiple HTTP server processes. It can only increase or decrease the
// size of the queue or read the current size of the queue, but not read
// or modify any other information about it.
//
// Both the mutation and read functionality is concurrency safe, but
// the read functionality is point-in-time only
type Counter interface {
	CountReader
	// Resize resizes the queue size by delta for the given host.
	Resize(host string, delta int) error
	// Ensure ensures that host is represented in this counter.
	// If host already has a nonzero value, then it is unchanged. If
	// it is missing, it is set to 0.
	Ensure(host string)
	// Remove tries to remove the given host and its
	// associated counts from the queue. returns true if it existed,
	// false otherwise.
	Remove(host string) bool
}

// Memory is a Counter implementation that
// holds the HTTP queue in memory only. Always use
// NewMemory to create one of these.
type Memory struct {
	countMap map[string]int
	mut      *sync.RWMutex
}

// NewMemoryQueue creates a new empty in-memory queue
func NewMemory() *Memory {
	lock := new(sync.RWMutex)
	return &Memory{
		countMap: make(map[string]int),
		mut:      lock,
	}
}

// Resize changes the size of the queue. Further calls to Current() return
// the newly calculated size if no other Resize() calls were made in the
// interim.
func (r *Memory) Resize(host string, delta int) error {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.countMap[host] += delta
	return nil
}

func (r *Memory) Ensure(host string) {
	r.mut.Lock()
	defer r.mut.Unlock()
	_, ok := r.countMap[host]
	if !ok {
		r.countMap[host] = 0
	}
}

func (r *Memory) Remove(host string) bool {
	r.mut.Lock()
	defer r.mut.Unlock()
	_, ok := r.countMap[host]
	delete(r.countMap, host)
	return ok
}

// Current returns the current size of the queue.
func (r *Memory) Current() (*Counts, error) {
	r.mut.RLock()
	defer r.mut.RUnlock()
	cts := NewCounts()
	cts.Counts = r.countMap
	return cts, nil
}
