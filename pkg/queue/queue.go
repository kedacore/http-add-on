package queue

import (
	"sync"
	"sync/atomic"
)

// CountReader represents the size of a virtual HTTP queue, possibly
// distributed across multiple HTTP server processes. It only can access
// the current size of the queue, not any other information about requests.
//
// It is concurrency safe.
type CountReader interface {
	// Current returns the current counts of pending requests.
	Current() (Counts, error)
}

// Counter represents a virtual HTTP queue, possibly distributed across
// multiple HTTP server processes. It can only increase or decrease the
// size of the queue or read the current size of the queue, but not read
// or modify any other information about it.
//
// Both the mutation and read functionality is concurrency safe, but
// the read functionality is point-in-time only.
type Counter interface {
	CountReader
	// Increase increases the queue size by delta for the given host.
	Increase(host string, delta int) error
	// Decrease decreases the queue size by delta for the given host.
	Decrease(host string, delta int) error
	// EnsureKey ensures that host is represented in this counter.
	EnsureKey(host string)
	// RemoveKey tries to remove the given host and its
	// associated counts from the queue. returns true if it existed,
	// false otherwise.
	RemoveKey(host string) bool
}

// Memory implements Counter and CountReader
var (
	_ Counter     = (*Memory)(nil)
	_ CountReader = (*Memory)(nil)
)

// hostEntry holds per-host state: an atomic concurrency counter
// and a monotonically-increasing request counter.
type hostEntry struct {
	concurrency  atomic.Int64
	requestCount atomic.Int64
}

// Memory is a Counter implementation that
// holds the HTTP queue in memory only. Always use
// NewMemory to create one of these.
//
// Hot-path guarantees:
//   - Increase: one sync.Map load + two atomic ops (no global lock).
//   - Decrease: one sync.Map load + one atomic CAS (no global lock).
type Memory struct {
	entries sync.Map // string -> *hostEntry
}

// NewMemory creates a new empty in-memory queue
func NewMemory() *Memory {
	return &Memory{}
}

// Increase atomically increments the concurrency counter and the
// monotonic request counter for host.
func (r *Memory) Increase(host string, delta int) error {
	if v, ok := r.entries.Load(host); ok {
		entry := v.(*hostEntry)
		entry.concurrency.Add(int64(delta))
		entry.requestCount.Add(int64(delta))
	}
	return nil
}

// Decrease atomically decrements the concurrency counter for host,
// clamped to zero.
func (r *Memory) Decrease(host string, delta int) error {
	if v, ok := r.entries.Load(host); ok {
		entry := v.(*hostEntry)
		for {
			cur := entry.concurrency.Load()
			if cur <= 0 {
				return nil
			}
			newVal := max(0, cur-int64(delta))

			if entry.concurrency.CompareAndSwap(cur, newVal) {
				return nil
			}
		}
	}
	return nil
}

// EnsureKey ensures that host is represented in this counter.
func (r *Memory) EnsureKey(host string) {
	if _, ok := r.entries.Load(host); ok {
		return
	}
	r.entries.LoadOrStore(host, &hostEntry{})
}

// RemoveKey tries to remove the given host and its associated counts
// from the queue. Returns true if it existed, false otherwise.
func (r *Memory) RemoveKey(host string) bool {
	_, existed := r.entries.LoadAndDelete(host)
	return existed
}

// Current returns the current size of the queue.
func (r *Memory) Current() (Counts, error) {
	cts := Counts{}
	r.entries.Range(func(k, v any) bool {
		key := k.(string)
		entry := v.(*hostEntry)

		cts[key] = Count{
			Concurrency:  int(entry.concurrency.Load()),
			RequestCount: entry.requestCount.Load(),
		}
		return true
	})
	return cts, nil
}
