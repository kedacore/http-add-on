package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/interceptor/config"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
)

// returns a kedanet.DialContextFunc by calling kedanet.DialContextWithRetry. if you pass nil for the
// timeoutConfig, it uses standard values. otherwise it uses the one you passed.
//
// the returned config.Timeouts is what was passed to the DialContextWithRetry function
func retryDialContextFunc(timeouts *config.Timeouts) (kedanet.DialContextFunc, *config.Timeouts) {
	if timeouts == nil {
		timeouts = &config.Timeouts{
			Connect:        100 * time.Millisecond,
			KeepAlive:      100 * time.Millisecond,
			ResponseHeader: 500 * time.Millisecond,
			TotalDial:      500 * time.Millisecond,
		}
	}
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	return kedanet.DialContextWithRetry(dialer, timeouts.TotalDial), timeouts
}

func reqAndRes(path string) (*httptest.ResponseRecorder, *http.Request, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err

	}
	resRecorder := httptest.NewRecorder()
	return resRecorder, req, nil
}

func TestForwardRequest(t *testing.T) {
	r := require.New(t)
	// this channel will be closed after the request was received, but
	// before the response was sent
	reqRecvCh := make(chan struct{})
	const respCode = 302
	const respBody = "TestForwardingHandler"
	originHdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		close(reqRecvCh)
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})
	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	dialCtxFunc, timeouts := retryDialContextFunc(nil)
	forwardRequest(
		res,
		req,
		dialCtxFunc,
		timeouts.ResponseHeader,
		forwardURL,
	)

	r.True(
		ensureSignalAfter(reqRecvCh, 100*time.Millisecond),
		"request was not received within %s",
		100*time.Millisecond,
	)
	forwardedRequests := originHdl.IncomingRequests()
	r.Equal(1, len(forwardedRequests), "number of requests forwarded")
	forwardedRequest := forwardedRequests[0]
	r.Equal(path, forwardedRequest.URL.Path)
	r.Equal(
		302,
		res.Code,
		"Proxied status code was wrong. Response body was %s",
		res.Body.String(),
	)
	r.Equal(respBody, res.Body.String())
}

// Test to make sure that the request forwarder times out if headers aren't returned in time
func TestHeaderTimeoutForwardRequest(t *testing.T) {
	r := require.New(t)
	// the origin will wait until this channel receives or is closed
	originWaitCh := make(chan struct{})
	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		<-originWaitCh
		w.WriteHeader(200)
	})
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	dialCtxFunc, timeouts := retryDialContextFunc(nil)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	forwardRequest(
		res,
		req,
		dialCtxFunc,
		timeouts.ResponseHeader,
		originURL,
	)

	forwardedRequests := hdl.IncomingRequests()
	r.Equal(0, len(forwardedRequests))
	r.Equal(502, res.Code)
	// the proxy has bailed out, so tell the origin to stop
	close(originWaitCh)

}
