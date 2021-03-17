package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// make sure the forwarding handler forwards normally with a
// reasonable forward timeout
func TestForwardingHandler(t *testing.T) {
	r := require.New(t)
	var wg sync.WaitGroup
	wg.Add(1)
	forwardedRequests := []*http.Request{}
	const respCode = 302
	const respBody = "TestForwardingHandler"
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		forwardedRequests = append(forwardedRequests, r)
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})
	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)

	r.NoError(err)
	handler := newForwardingHandler(forwardURL, 1*time.Second)
	const path = "/testfwd"
	req, err := http.NewRequest("GET", path, nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	r.Equal(1, len(forwardedRequests), "number of requests forwarded")
	forwardedRequest := forwardedRequests[0]
	r.Equal(path, forwardedRequest.URL.Path)
	r.Equal(302, rec.Code)
	r.Equal(respBody, rec.Body.String())
}

// make sure the forwarding handler doesn't forward if the origin didn't return
// within the hold timeout
func TestForwardingHandlerNoForward(t *testing.T) {
	r := require.New(t)
	const respCode = 400
	const respBody = "you shouldn't see this!"
	var wg sync.WaitGroup
	wg.Add(1)
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Wait()
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})

	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)
	handler := newForwardingHandler(forwardURL, 100*time.Millisecond)
	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// we should not get a 400 back from the proxy, even though that's
	// what the origin returns
	r.Equal(502, rec.Code, "response code from proxy")
	// nor should we get the handler's body back either. it should be
	// the proxy's error
	r.Contains(rec.Body.String(), "Error on backend")
	// tell the handler to be done so that we can close the server
	wg.Done()
}

// test to make sure that proxy will hold onto a request while an origin is working.
// note: this is _not_ the same as a test to hold the request while a connection is being
// established
func TestForwardingHandlerWaits(t *testing.T) {
	r := require.New(t)
	// have the origin sleep for less time than the interceptor
	// will wait
	const originSleep = 200 * time.Millisecond
	const proxySleep = 1 * time.Second

	const respCode = 400
	const respBody = "you shouldn't see this!"
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(originSleep)
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})

	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)
	handler := newForwardingHandler(forwardURL, proxySleep)

	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// we should not get a 400 back from the proxy, even though that's
	// what the origin returns
	r.Equal(respCode, rec.Code, "response code from proxy")
	// nor should we get the handler's body back either. it should be
	// the proxy's error
	r.Equal(respBody, rec.Body.String(), "response body from proxy")
}

// test to make sure that proxy will wait to try and establish a connection
// while an origin is not yet listening
func TestForwardingHandlerHolds(t *testing.T) {
	r := require.New(t)
	// have the origin sleep for less time than the interceptor
	// will wait
	// const originSleep = 200 * time.Millisecond
	const proxyTimeout = 1 * time.Second

	// const respCode = 400
	// const respBody = "you shouldn't see this!"
	// originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	time.Sleep(originSleep)
	// 	w.WriteHeader(respCode)
	// 	w.Write([]byte(respBody))
	// })

	// testServer := httptest.NewUnstartedServer(originHdl)
	// log.Printf("test server URL: %s", testServer.URL)
	// defer testServer.Close()
	// forwardURL, err := url.Parse(testServer.URL)
	forwardURL, err := url.Parse("https://asfgsdfgkjdfg.dev")
	r.NoError(err)
	handler := newForwardingHandler(forwardURL, proxyTimeout)

	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()

	reqStart := time.Now()
	handler.ServeHTTP(rec, req)
	reqDur := time.Since(reqStart)
	r.True(
		reqDur >= proxyTimeout,
		"request duration (%s) wasn't >= than the proxy's wait time (%s)",
		reqDur,
		proxyTimeout,
	)
}
