package main

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// QueueCounter tracks per-host concurrent request counts and RPS.
//
// Hot-path guarantees:
//   - Increase / Decrease: one sync.Map read + one atomic op (no global lock).
//   - RPS recording: one sync.Mutex lock *per host* (no global lock).
//
// This replaces the old implementation that used a single global sync.RWMutex.
type QueueCounter struct {
	entries sync.Map // string -> *hostEntry
}

// QueueGuard decrements the in-flight counter when released.
// Must be released exactly once (typically via defer).
type QueueGuard struct {
	queue *QueueCounter
	key   string
}

// Release decrements the concurrency counter. Safe to call multiple times
// (only the first call has an effect).
func (g *QueueGuard) Release() {
	if g.queue != nil {
		g.queue.decrease(g.key)
		g.queue = nil
	}
}

// QueueCount is the wire format for the /queue endpoint.
// Field names must match the Go JSON contract expected by the Scaler.
type QueueCount struct {
	Concurrency int     `json:"Concurrency"`
	RPS         float64 `json:"RPS"`
}

// NewQueueCounter creates a new empty queue counter.
func NewQueueCounter() *QueueCounter {
	return &QueueCounter{}
}

// EnsureKey ensures a host entry exists (idempotent).
func (q *QueueCounter) EnsureKey(key string) {
	q.entries.LoadOrStore(key, newHostEntry())
}

// RemoveKey removes a host entry.
func (q *QueueCounter) RemoveKey(key string) {
	q.entries.Delete(key)
}

// RetainKeys removes all keys not in the given set.
func (q *QueueCounter) RetainKeys(keys map[string]struct{}) {
	q.entries.Range(func(k, _ any) bool {
		if _, ok := keys[k.(string)]; !ok {
			q.entries.Delete(k)
		}
		return true
	})
}

// UpdateBuckets creates or replaces the RPS ring-buffer for a key.
func (q *QueueCounter) UpdateBuckets(key string, window, granularity time.Duration) {
	if v, ok := q.entries.Load(key); ok {
		entry := v.(*hostEntry)
		entry.mu.Lock()
		entry.buckets = newRPSBuckets(window, granularity)
		entry.mu.Unlock()
	}
}

// Increase atomically increments the concurrency counter for key and
// records an RPS data-point. Returns a QueueGuard that decrements on Release.
func (q *QueueCounter) Increase(key string) *QueueGuard {
	if v, ok := q.entries.Load(key); ok {
		entry := v.(*hostEntry)
		entry.concurrency.Add(1)

		// Record RPS data-point (per-host lock, not global)
		entry.mu.Lock()
		if entry.buckets != nil {
			entry.buckets.record(time.Now(), 1.0)
		}
		entry.mu.Unlock()
	}
	return &QueueGuard{queue: q, key: key}
}

// Current returns a snapshot of all per-host counts. Called by GET /queue.
func (q *QueueCounter) Current() map[string]QueueCount {
	now := time.Now()
	result := make(map[string]QueueCount)
	q.entries.Range(func(k, v any) bool {
		key := k.(string)
		entry := v.(*hostEntry)
		concurrency := entry.concurrency.Load()

		entry.mu.Lock()
		rps := 0.0
		if entry.buckets != nil {
			rps = entry.buckets.windowAverage(now)
		}
		entry.mu.Unlock()

		result[key] = QueueCount{
			Concurrency: int(concurrency),
			RPS:         rps,
		}
		return true
	})
	return result
}

// CurrentJSON returns the queue counts as JSON bytes, ready for the /queue response.
func (q *QueueCounter) CurrentJSON() ([]byte, error) {
	return json.Marshal(q.Current())
}

// decrease atomically decrements the concurrency counter (clamped to 0).
func (q *QueueCounter) decrease(key string) {
	if v, ok := q.entries.Load(key); ok {
		entry := v.(*hostEntry)
		for {
			cur := entry.concurrency.Load()
			if cur <= 0 {
				return
			}
			if entry.concurrency.CompareAndSwap(cur, cur-1) {
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Per-host entry
// ---------------------------------------------------------------------------

type hostEntry struct {
	concurrency atomic.Int64
	mu          sync.Mutex  // protects buckets only
	buckets     *rpsBuckets // nil until UpdateBuckets is called
}

func newHostEntry() *hostEntry {
	return &hostEntry{}
}

// ---------------------------------------------------------------------------
// RPS ring-buffer (Knative-inspired)
// ---------------------------------------------------------------------------

type rpsBuckets struct {
	data          []float64
	window        time.Duration
	granularity   time.Duration
	lastWriteTime time.Time
	lastWriteIdx  int
}

func newRPSBuckets(window, granularity time.Duration) *rpsBuckets {
	if granularity <= 0 {
		granularity = time.Second
	}
	n := int(window / granularity)
	if n < 1 {
		n = 1
	}
	return &rpsBuckets{
		data:          make([]float64, n),
		window:        window,
		granularity:   granularity,
		lastWriteTime: time.Now(),
		lastWriteIdx:  0,
	}
}

func (b *rpsBuckets) record(now time.Time, delta float64) {
	b.advanceTo(now)
	idx := b.bucketIndex(now)
	b.data[idx] += delta
	b.lastWriteTime = now
	b.lastWriteIdx = idx
}

func (b *rpsBuckets) windowAverage(now time.Time) float64 {
	windowStart := now.Add(-b.window)
	if b.lastWriteTime.Before(windowStart) {
		return 0 // all data is stale
	}
	total := 0.0
	for _, v := range b.data {
		total += v
	}
	secs := b.window.Seconds()
	if secs > 0 {
		return total / secs
	}
	return 0
}

func (b *rpsBuckets) bucketIndex(now time.Time) int {
	elapsed := now.Sub(b.lastWriteTime)
	steps := int(elapsed / b.granularity)
	return (b.lastWriteIdx + steps) % len(b.data)
}

func (b *rpsBuckets) advanceTo(now time.Time) {
	elapsed := now.Sub(b.lastWriteTime)
	steps := int(elapsed / b.granularity)
	if steps == 0 {
		return
	}
	clearCount := steps
	if clearCount > len(b.data) {
		clearCount = len(b.data)
	}
	for i := 1; i <= clearCount; i++ {
		idx := (b.lastWriteIdx + i) % len(b.data)
		b.data[idx] = 0
	}
}
