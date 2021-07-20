package queue

var _ Counter = &FakeCounter{}

type FakeCounter struct {
	ResizedCh chan int
}

func NewFakeCounter() *FakeCounter {
	return &FakeCounter{
		ResizedCh: make(chan int),
	}
}

func (f *FakeCounter) Resize(host string, i int) error {
	f.ResizedCh <- i
	return nil
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
