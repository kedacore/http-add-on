package routing

import "sync"

type TableVersionReader interface {
	VersionHistory() ([]string, error)
}

type TableVersionWriter interface {
	AddVersion(string) error
}

type TableVersionReaderWriter interface {
	TableVersionReader
	TableVersionWriter
}

type TableVersionHistory struct {
	rwm  *sync.RWMutex
	hist []string
}

var _ TableVersionReaderWriter = &TableVersionHistory{}

func NewTableVersionHistory() *TableVersionHistory {
	return &TableVersionHistory{
		rwm:  &sync.RWMutex{},
		hist: []string{},
	}
}

func (t *TableVersionHistory) VersionHistory() ([]string, error) {
	t.rwm.RLock()
	defer t.rwm.RUnlock()
	return t.hist, nil
}

func (t *TableVersionHistory) AddVersion(v string) error {
	t.rwm.Lock()
	defer t.rwm.Unlock()
	t.hist = append(t.hist, v)
	return nil
}
