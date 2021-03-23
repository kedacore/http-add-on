package net

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
)

type TestHTTPHandlerWrapper struct {
	rwm              *sync.RWMutex
	hdl              http.Handler
	incomingRequests []http.Request
}

func (t *TestHTTPHandlerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.hdl.ServeHTTP(w, r)
	// log a request after the handler has been run
	t.rwm.Lock()
	t.incomingRequests = append(t.incomingRequests, *r)
	// do not put this unlock in a defer or after the ServeHTTP line below,
	// or you risk deadlock if the handler waits for something or otherwise
	// hangs
	t.rwm.Unlock()
}

// IncomingRequests returns a copy slice of all the requests that have been received before
// this function was called.
func (t *TestHTTPHandlerWrapper) IncomingRequests() []http.Request {
	t.rwm.RLock()
	defer t.rwm.RUnlock()
	retSlice := make([]http.Request, len(t.incomingRequests))
	copy(retSlice, t.incomingRequests)
	return retSlice
}

func NewTestHTTPHandlerWrapper(hdl http.HandlerFunc) *TestHTTPHandlerWrapper {
	return &TestHTTPHandlerWrapper{
		rwm:              new(sync.RWMutex),
		hdl:              hdl,
		incomingRequests: nil,
	}
}

// StartTestServer starts an *httptest.Server, parses its URL, and returns both values.
// The caller is responsible for closing the returned server
func StartTestServer(hdl http.Handler) (*httptest.Server, *url.URL, error) {
	srv := httptest.NewServer(hdl)
	u, err := url.Parse(srv.URL)
	if err != nil {
		return nil, nil, err
	}
	return srv, u, nil
}
