package routing

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
)

var ErrTargetNotFound = errors.New("Target not found")

type Target struct {
	Service    string
	Port       int
	Deployment string
}

func (t *Target) ServiceURL() (*url.URL, error) {
	urlStr := fmt.Sprintf("http://%s:%d", t.Service, t.Port)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil

}

type Table struct {
	m map[string]Target
	l *sync.RWMutex
}

func NewTable() *Table {
	return &Table{
		m: make(map[string]Target),
		l: new(sync.RWMutex),
	}
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
