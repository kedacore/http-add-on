package queue

import (
	"fmt"
	"time"
)

var _ Counter = &FakeCounter{}

type HostAndCount struct {
	Host  string
	Count int
}
type FakeCounter struct {
	RetMap        map[string]int
	ResizedCh     chan HostAndCount
	ResizeTimeout time.Duration
}

func NewFakeCounter() *FakeCounter {
	return &FakeCounter{
		RetMap:        map[string]int{},
		ResizedCh:     make(chan HostAndCount),
		ResizeTimeout: 1 * time.Second,
	}
}

func (f *FakeCounter) Resize(host string, i int) error {
	f.RetMap[host] = i
	select {
	case f.ResizedCh <- HostAndCount{Host: host, Count: i}:
	case <-time.After(f.ResizeTimeout):
		return fmt.Errorf(
			"FakeCounter.Resize timeout after %s",
			f.ResizeTimeout,
		)
	}
	return nil
}

func (f *FakeCounter) Ensure(host string) {
	f.RetMap[host] = 0
}

func (f *FakeCounter) Remove(host string) bool {
	_, ok := f.RetMap[host]
	delete(f.RetMap, host)
	return ok
}

func (f *FakeCounter) Current() (*Counts, error) {
	ret := NewCounts()
	retMap := f.RetMap
	if len(retMap) == 0 {
		retMap["sample.com"] = 0
	}
	ret.Counts = retMap
	return ret, nil
}

var _ CountReader = &FakeCountReader{}

type FakeCountReader struct {
	current int
	err     error
}

func (f *FakeCountReader) Current() (*Counts, error) {
	ret := NewCounts()
	ret.Counts = map[string]int{
		"sample.com": f.current,
	}
	return ret, f.err
}
