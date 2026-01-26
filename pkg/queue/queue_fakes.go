package queue

import (
	"fmt"
	"sync"
	"time"
)

var _ Counter = (*FakeCounter)(nil)

type HostAndCount struct {
	Host  string
	Count int
}
type FakeCounter struct {
	mapMut        *sync.RWMutex
	RetMap        map[string]Count
	ResizedCh     chan HostAndCount
	ResizeTimeout time.Duration
}

// NewFakeCounter creates a FakeCounter with an unbuffered channel.
// Use this when tests need to synchronize on queue events.
func NewFakeCounter() *FakeCounter {
	return newFakeCounter(0)
}

// NewFakeCounterBuffered creates a FakeCounter with a buffered channel.
// Use this when tests don't need to synchronize on queue events.
// The buffered channel prevents blocking on Increase/Decrease calls.
func NewFakeCounterBuffered() *FakeCounter {
	return newFakeCounter(100)
}

func newFakeCounter(bufferSize int) *FakeCounter {
	return &FakeCounter{
		mapMut:        new(sync.RWMutex),
		RetMap:        map[string]Count{},
		ResizedCh:     make(chan HostAndCount, bufferSize),
		ResizeTimeout: 1 * time.Second,
	}
}

func (f *FakeCounter) Increase(host string, i int) error {
	f.mapMut.Lock()
	count := f.RetMap[host]
	count.Concurrency += i
	count.RPS += float64(i)
	f.RetMap[host] = count
	f.mapMut.Unlock()
	select {
	case f.ResizedCh <- HostAndCount{Host: host, Count: i}:
	case <-time.After(f.ResizeTimeout):
		return fmt.Errorf(
			"FakeCounter.Increase timeout after %s",
			f.ResizeTimeout,
		)
	}
	return nil
}

func (f *FakeCounter) Decrease(host string, i int) error {
	f.mapMut.Lock()
	count := f.RetMap[host]
	count.Concurrency -= i
	f.RetMap[host] = count
	f.mapMut.Unlock()
	select {
	case f.ResizedCh <- HostAndCount{Host: host, Count: i}:
	case <-time.After(f.ResizeTimeout):
		return fmt.Errorf(
			"FakeCounter.Decrease timeout after %s",
			f.ResizeTimeout,
		)
	}
	return nil
}

func (f *FakeCounter) EnsureKey(host string, _, _ time.Duration) {
	f.mapMut.Lock()
	defer f.mapMut.Unlock()
	f.RetMap[host] = Count{
		Concurrency: 0,
	}
}

func (f *FakeCounter) UpdateBuckets(host string, window, granularity time.Duration) {
	f.EnsureKey(host, window, granularity)
}

func (f *FakeCounter) RemoveKey(host string) bool {
	f.mapMut.Lock()
	defer f.mapMut.Unlock()
	_, ok := f.RetMap[host]
	delete(f.RetMap, host)
	return ok
}

func (f *FakeCounter) Current() (*Counts, error) {
	ret := NewCounts()
	f.mapMut.RLock()
	defer f.mapMut.RUnlock()
	retMap := f.RetMap
	ret.Counts = retMap
	return ret, nil
}

var _ CountReader = &FakeCountReader{}

type FakeCountReader struct {
	concurrency int
	rps         float64
	err         error
}

func (f *FakeCountReader) Current() (*Counts, error) {
	ret := NewCounts()
	ret.Counts = map[string]Count{
		"sample.com": {
			Concurrency: f.concurrency,
			RPS:         f.rps,
		},
	}
	return ret, f.err
}
