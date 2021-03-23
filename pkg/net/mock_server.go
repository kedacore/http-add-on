package net

import (
	"net/http"
	"sync"
)

type TestHTTPHandlerWrapper struct {
	rwm              *sync.RWMutex
	hdl              http.Handler
	incomingRequests []*http.Request
}

func (t *TestHTTPHandlerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.rwm.Lock()
	t.incomingRequests = append(t.incomingRequests, r)
	t.rwm.Unlock()
	t.rwm.RLock()
	defer t.rwm.RUnlock()
	t.hdl.ServeHTTP(w, r)
}

func NewTestHTTPHandlerWrapper(hdl http.HandlerFunc) *TestHTTPHandlerWrapper {
	return &TestHTTPHandlerWrapper{
		rwm:              new(sync.RWMutex),
		hdl:              hdl,
		incomingRequests: nil,
	}
}
