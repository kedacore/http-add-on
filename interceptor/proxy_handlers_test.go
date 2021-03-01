package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
)

func (i *InterceptorSuite) TestForwardingHandler() {
	const originReturn = "hello test"
	forwardedRequests := []*http.Request{}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedRequests = append(forwardedRequests, r)
		w.Write([]byte(originReturn))
	}))
	defer testServer.Close()

	forwardURL, err := url.Parse(testServer.URL)
	i.NoError(err)
	handler := newForwardingHandler(forwardURL)
	const path = "/testfwd"
	_, echoCtx, rec := newTestCtx("GET", path)
	i.NoError(handler(echoCtx))
	i.Equal(1, len(forwardedRequests), "number of requests that were forwarded")
	forwardedRequest := forwardedRequests[0]
	i.Equal(path, forwardedRequest.URL.Path)
	i.Equal(originReturn, string(rec.Body.Bytes()))
}
