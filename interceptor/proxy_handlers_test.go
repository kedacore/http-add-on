package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
)

// the proxy should successfully forward a request to a running server
func TestImmediatelySuccessfulProxy(t *testing.T) {
	const host = "TestImmediatelySuccessfulProxy.testing"
	r := require.New(t)

	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("test response"))
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer srv.Close()
	routingTable := routing.NewTable()
	portInt, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	target := routing.Target{
		Service:    strings.Split(originURL.Host, ":")[0],
		Port:       portInt,
		Deployment: "testdepl",
	}
	routingTable.AddTarget(host, target)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func(deployName string) error {
		return nil
	}
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	req.Host = host
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal(200, res.Code, "expected response code 200")
	r.Equal("test response", res.Body.String())
}

// the proxy should wait for a timeout and fail if there is no origin to connect
// to
func TestWaitFailedConnection(t *testing.T) {
	const host = "TestWaitFailedConnection.testing"
	r := require.New(t)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func(deplName string) error {
		return nil
	}
	routingTable := routing.NewTable()
	routingTable.AddTarget(host, routing.Target{
		Service:    "nosuchdepl",
		Port:       8081,
		Deployment: "nosuchdepl",
	})
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	req.Host = host
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal(502, res.Code, "response code was unexpected")
}

// the proxy handler should wait for the wait function until it hits
// a timeout, then it should fail
func TestTimesOutOnWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	timeouts.DeploymentReplicas = 10 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	waitFunc, waitFuncCalledCh, finishWaitFunc := notifyingFunc()
	start := time.Now()
	waitDur := timeouts.DeploymentReplicas * 2
	go func() {
		time.Sleep(waitDur)
		finishWaitFunc()
	}()
	noSuchHost := "TestTimesOutOnWaitFunc.testing"

	routingTable := routing.NewTable()
	routingTable.AddTarget(noSuchHost, routing.Target{
		Service:    "nosuchsvc",
		Port:       9091,
		Deployment: "nosuchdepl",
	})
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = noSuchHost

	hdl.ServeHTTP(res, req)
	r.NoError(waitForSignal(waitFuncCalledCh, 1*time.Second))
	r.GreaterOrEqual(time.Since(start), waitDur)
	r.Equal(502, res.Code, "response code was unexpected")
}

// Test to make sure the proxy handler will wait for the waitFunc to
// complete
func TestWaitsForWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	waitFunc, waitFuncCalledCh, finishWaitFunc := notifyingFunc()
	noSuchHost := "TestWaitsForWaitFunc.test"
	routingTable := routing.NewTable()
	routingTable.AddTarget(noSuchHost, routing.Target{
		Service:    "nosuchsvc",
		Port:       9092,
		Deployment: "nosuchdepl",
	})
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = noSuchHost

	start := time.Now()
	waitDur := 10 * time.Millisecond

	// make the wait function finish after a short duration
	go func() {
		time.Sleep(waitDur)
		finishWaitFunc()
	}()
	hdl.ServeHTTP(res, req)
	r.NoError(waitForSignal(waitFuncCalledCh, 1*time.Second))
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
	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-originHdlCh
			w.WriteHeader(200)
			w.Write([]byte("test response"))
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func(deplName string) error {
		return nil
	}
	routingTable := routing.NewTable()
	target := routing.Target{
		Service:    "testsvc",
		Port:       9094,
		Deployment: "testdepl",
	}
	routingTable.AddTarget(originURL.Host, target)
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = originURL.Host

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

func waitForSignal(sig <-chan struct{}, waitDur time.Duration) error {
	tmr := time.NewTimer(waitDur)
	defer tmr.Stop()
	select {
	case <-sig:
		return nil
	case <-tmr.C:
		return fmt.Errorf("signal didn't happen within %s", waitDur)
	}
}

// notifyingFunc creates a new function to be used as a waitFunc in the
// newForwardingHandler function. it also returns a channel that will
// be closed immediately after the function is called (not necessarily
// before it returns). the function won't return until the returned func()
// is called
func notifyingFunc() (func(string) error, <-chan struct{}, func()) {
	calledCh := make(chan struct{})
	finishCh := make(chan struct{})
	finishFunc := func() {
		close(finishCh)
	}
	return func(string) error {
		close(calledCh)
		<-finishCh
		return nil
	}, calledCh, finishFunc
}
