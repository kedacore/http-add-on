package queue

import (
	"sync"
	"sync/atomic"
	"time"
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
	// Increase increases the queue size by delta for the given host.
	Increase(host string, delta int) error
	// Decrease decreases the queue size by delta for the given host.
	Decrease(host string, delta int) error
	// EnsureKey ensures that host is represented in this counter.
	EnsureKey(host string, window, granularity time.Duration)
	// UpdateBuckets update request backets if there are changes
	UpdateBuckets(host string, window, granularity time.Duration)
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
// and an atomically-swappable RPS ring-buffer. This eliminates the global
// lock that previously serialized all Increase/Decrease calls.
// RequestsBuckets has its own internal mutex, so no outer lock is needed.
type hostEntry struct {
	concurrency atomic.Int64
	buckets     atomic.Pointer[RequestsBuckets]
}

// Memory is a Counter implementation that
// holds the HTTP queue in memory only. Always use
// NewMemory to create one of these.
//
// Hot-path guarantees:
//   - Increase / Decrease: one sync.Map load + one atomic op (no global lock).
//   - RPS recording uses the RequestsBuckets internal lock only.
type Memory struct {
	entries sync.Map // string -> *hostEntry
}

// NewMemoryQueue creates a new empty in-memory queue
func NewMemory() *Memory {
	return &Memory{}
}

// Increase atomically increments the concurrency counter for host
// and records an RPS data-point.
func (r *Memory) Increase(host string, delta int) error {
	if v, ok := r.entries.Load(host); ok {
		entry := v.(*hostEntry)
		entry.concurrency.Add(int64(delta))

		if b := entry.buckets.Load(); b != nil {
			b.Record(time.Now(), delta)
		}
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

func (r *Memory) EnsureKey(host string, window, granularity time.Duration) {
	if _, ok := r.entries.Load(host); ok {
		return
	}
	entry := &hostEntry{}
	entry.buckets.Store(NewRequestsBuckets(window, granularity))
	r.entries.LoadOrStore(host, entry)
}

func (r *Memory) UpdateBuckets(host string, window, granularity time.Duration) {
	r.EnsureKey(host, window, granularity)
	if v, ok := r.entries.Load(host); ok {
		entry := v.(*hostEntry)
		if b := entry.buckets.Load(); b != nil &&
			(b.window != window || b.granularity != granularity) {
			entry.buckets.Store(NewRequestsBuckets(window, granularity))
		}
	}
}

func (r *Memory) RemoveKey(host string) bool {
	_, existed := r.entries.LoadAndDelete(host)
	return existed
}

// Current returns the current size of the queue.
func (r *Memory) Current() (*Counts, error) {
	now := time.Now()
	cts := NewCounts()
	r.entries.Range(func(k, v any) bool {
		key := k.(string)
		entry := v.(*hostEntry)
		concurrency := entry.concurrency.Load()

		rps := 0.0
		if b := entry.buckets.Load(); b != nil {
			rps = b.WindowAverage(now)
		}

		cts.Counts[key] = Count{
			Concurrency: int(concurrency),
			RPS:         rps,
		}
		return true
	})
	return cts, nil
}
