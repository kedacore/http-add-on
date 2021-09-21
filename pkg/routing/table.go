package routing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

type TableWriter interface {
	json.Unmarshaler
	AddTarget(host string, target Target) error
	RemoveTarget(host string) error
}

// TableReader is a table that can only be read from. This interface
// is useful to accept as a parameter in functions that only need to read
// from a table

type TableReader interface {
	json.Marshaler
	Lookup(host string) (Target, error)
	Hosts() []string
}

// TableReaderWriter is a table reader and writer
type TableReaderWriter interface {
	TableReader
	TableWriter
}

// Table is an in-memory routing table that implements
// TableReaderWriter.

type Table struct {
	json.Marshaler
	json.Unmarshaler
	fmt.Stringer
	m           map[string]Target
	l           *sync.RWMutex
	versionHist []string
}

func NewTable() *Table {
	return &Table{
		m:           make(map[string]Target),
		l:           new(sync.RWMutex),
		versionHist: []string{},
	}
}

func (t *Table) String() string {
	t.l.RLock()
	defer t.l.RUnlock()
	return fmt.Sprintf("%v", t.m)
}

func (t *Table) MarshalJSON() ([]byte, error) {
	t.l.RLock()
	defer t.l.RUnlock()
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(t.m)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (t *Table) UnmarshalJSON(data []byte) error {
	t.l.Lock()
	defer t.l.Unlock()
	t.m = map[string]Target{}
	b := bytes.NewBuffer(data)
	return json.NewDecoder(b).Decode(&t.m)
}

func (t *Table) Lookup(host string) (Target, error) {
	t.l.RLock()
	defer t.l.RUnlock()
	ret, ok := t.m[host]
	if !ok {
		return Target{}, ErrTargetNotFound
	}
	return ret, nil
}

// AddTarget registers target for host in the routing table
// and returns nil if that host didn't already exist.
// Returns a non-nil error if it did already exist.
func (t *Table) AddTarget(
	host string,
	target Target,
) error {
	t.l.Lock()
	defer t.l.Unlock()
	_, ok := t.m[host]
	if ok {
		return fmt.Errorf(
			"host %s is already registered in the routing table",
			host,
		)
	}
	t.m[host] = target
	return nil
}

// RemoveTarget removes host and its corresponding target
// if host already existed in the table, and returns nil.
//
// Returns a non-nil error if the host wasn't already in the
// table.
func (t *Table) RemoveTarget(host string) error {
	t.l.Lock()
	defer t.l.Unlock()
	_, ok := t.m[host]
	if !ok {
		return fmt.Errorf("host %s did not exist in the routing table", host)
	}
	delete(t.m, host)
	return nil
}

func (t *Table) Hosts() []string {
	t.l.RLock()
	defer t.l.RUnlock()
	ret := make([]string, 0, len(t.m))
	for host := range t.m {
		ret = append(ret, host)
	}
	return ret
}
