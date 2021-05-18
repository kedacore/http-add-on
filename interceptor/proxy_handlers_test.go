package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
)

// the proxy should successfully forward a request to a running server
func TestImmediatelySuccessfulProxy(t *testing.T) {
	r := require.New(t)

	originHdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("test response"))
	})
	srv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func() error {
		return nil
	}
	hdl := newForwardingHandler(
		originURL,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal(200, res.Code, "response code was unexpected")
	r.Equal("test response", res.Body.String())
}

// the proxy should wait for a timeout and fail if there is no origin to connect
// to
func TestWaitFailedConnection(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func() error {
		return nil
	}
	noSuchURL, err := url.Parse("http://localhost:60002")
	r.NoError(err)
	hdl := newForwardingHandler(
		noSuchURL,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal(502, res.Code, "response code was unexpected")
}

func TestTimesOutOnWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	timeouts.DeploymentReplicas = 10 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	// the wait func will close this channel immediately after it's called, but before it starts
	// waiting for waitFuncCh
	waitFuncCalledCh := make(chan struct{})
	// the wait func will wait for waitFuncCh to receive or be closed before it proceeds
	waitFuncCh := make(chan struct{})
	waitFunc := func() error {
		close(waitFuncCalledCh)
		<-waitFuncCh
		return nil
	}
	noSuchURL, err := url.Parse("http://localhost:60002")
	r.NoError(err)
	hdl := newForwardingHandler(
		noSuchURL,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)

	start := time.Now()
	waitDur := timeouts.DeploymentReplicas * 2
	go func() {
		time.Sleep(waitDur)
		close(waitFuncCh)
	}()
	hdl.ServeHTTP(res, req)
	select {
	case <-waitFuncCalledCh:
	case <-time.After(1 * time.Second):
		r.Fail("the wait function wasn't called")
	}
	r.GreaterOrEqual(time.Since(start), waitDur)

	r.Equal(502, res.Code, "response code was unexpected")
}

func TestWaitsForWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	// the wait func will close this channel immediately after it's called, but before it starts
	// waiting for waitFuncCh
	waitFuncCalledCh := make(chan struct{})
	// the wait func will wait for waitFuncCh to receive or be closed before it proceeds
	waitFuncCh := make(chan struct{})
	waitFunc := func() error {
		close(waitFuncCalledCh)
		<-waitFuncCh
		return nil
	}
	noSuchURL, err := url.Parse("http://localhost:60002")
	r.NoError(err)
	hdl := newForwardingHandler(
		noSuchURL,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)

	start := time.Now()
	waitDur := 10 * time.Millisecond
	go func() {
		time.Sleep(waitDur)
		close(waitFuncCh)
	}()
	hdl.ServeHTTP(res, req)
	select {
	case <-waitFuncCalledCh:
	case <-time.After(1 * time.Second):
		r.Fail("the wait function wasn't called")
	}
	r.GreaterOrEqual(time.Since(start), waitDur)

	r.Equal(502, res.Code, "response code was unexpected")
}

// the proxy should connect to a server, and then time out if the server doesn't
// respond in time
func TestWaitHeaderTimeout(t *testing.T) {
	r := require.New(t)

	// the origin will wait for this channel to receive or close before it sends any data back to the
	// proxy
	originHdlCh := make(chan struct{})
	originHdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		<-originHdlCh
		w.WriteHeader(200)
		w.Write([]byte("test response"))
	})
	srv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func() error {
		return nil
	}
	hdl := newForwardingHandler(
		originURL,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal(502, res.Code, "response code was unexpected")
	close(originHdlCh)
}

// ensureSignalAfter returns true if signalCh receives before timeout, false otherwise.
// it blocks for timeout at most
func ensureSignalBeforeTimeout(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-signalCh:
		return true
	}
}
