package queue

var _ Counter = &FakeCounter{}

type HostAndCount struct {
	Host  string
	Count int
}
type FakeCounter struct {
	ResizedCh chan HostAndCount
}

func NewFakeCounter() *FakeCounter {
	return &FakeCounter{
		ResizedCh: make(chan HostAndCount),
	}
}

func (f *FakeCounter) Resize(host string, i int) error {
	f.ResizedCh <- HostAndCount{Host: host, Count: i}
	return nil
}

func (f *FakeCounter) Ensure(host string) {
}

func (f *FakeCounter) Current() (*Counts, error) {
	ret := NewCounts()
	ret.Counts = map[string]int{
		"sample.com": 0,
	}
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
