package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	const ns = "testns"
	host := fmt.Sprintf("%s.testing", t.Name())
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
	originPort, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	target := targetFromURL(
		ns,
		originURL,
		originPort,
		"testdepl",
		123,
	)
	routingTable.AddTarget(host, target)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	waitFunc := func(context.Context, string, string) (int, error) {
		return 1, nil
	}
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		func(routing.Target) (*url.URL, error) {
			return originURL, nil
		},
		forwardingConfig{
			waitTimeout:       timeouts.DeploymentReplicas,
			respHeaderTimeout: timeouts.ResponseHeader,
		},
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	req.Host = host
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal("false", res.Header().Get("X-KEDA-HTTP-Cold-Start"), "expected X-KEDA-HTTP-Cold-Start false")
	r.Equal(200, res.Code, "expected response code 200")
	r.Equal("test response", res.Body.String())
}

// the proxy should wait for a timeout and fail if there is no
// origin to which to connect
func TestWaitFailedConnection(t *testing.T) {
	const host = "TestWaitFailedConnection.testing"
	r := require.New(t)

	timeouts := defaultTimeouts()
	backoff := timeouts.DefaultBackoff()
	backoff.Steps = 2
	dialCtxFunc := retryDialContextFunc(
		timeouts,
		backoff,
	)
	waitFunc := func(context.Context, string, string) (int, error) {
		return 1, nil
	}
	routingTable := routing.NewTable()
	routingTable.AddTarget(host, routing.NewTarget(
		"testns",
		"nosuchdepl",
		8081,
		"nosuchdepl",
		1234,
	))

	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		func(routing.Target) (*url.URL, error) {
			return url.Parse("http://nosuchhost:8081")
		},
		forwardingConfig{
			waitTimeout:       timeouts.DeploymentReplicas,
			respHeaderTimeout: timeouts.ResponseHeader,
		},
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	req.Host = host
	r.NoError(err)

	hdl.ServeHTTP(res, req)

	r.Equal("false", res.Header().Get("X-KEDA-HTTP-Cold-Start"), "expected X-KEDA-HTTP-Cold-Start false")
	r.Equal(502, res.Code, "response code was unexpected")
}

// the proxy handler should wait for the wait function until it hits
// a timeout, then it should fail
func TestTimesOutOnWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	timeouts.DeploymentReplicas = 1 * time.Millisecond
	timeouts.ResponseHeader = 1 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	waitFunc, waitFuncCalledCh, finishWaitFunc := notifyingFunc()
	defer finishWaitFunc()
	noSuchHost := fmt.Sprintf("%s.testing", t.Name())

	routingTable := routing.NewTable()
	routingTable.AddTarget(noSuchHost, routing.NewTarget(
		"testns",
		"nosuchsvc",
		9091,
		"nosuchdepl",
		1234,
	))
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		func(routing.Target) (*url.URL, error) {
			return url.Parse("http://nosuchhost:9091")
		},
		forwardingConfig{
			waitTimeout:       timeouts.DeploymentReplicas,
			respHeaderTimeout: timeouts.ResponseHeader,
		},
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = noSuchHost

	start := time.Now()
	hdl.ServeHTTP(res, req)
	elapsed := time.Since(start)

	t.Logf("elapsed time was %s", elapsed)
	// serving should take at least timeouts.DeploymentReplicas, but no more than
	// timeouts.DeploymentReplicas*4
	r.GreaterOrEqual(elapsed, timeouts.DeploymentReplicas)
	r.LessOrEqual(elapsed, timeouts.DeploymentReplicas*4)
	r.Equal(502, res.Code, "response code was unexpected")

	// we will always return the X-KEDA-HTTP-Cold-Start header
	// when we are able to forward the
	// request to the backend but not if we have failed due
	// to a timeout from a waitFunc or earlier in the pipeline,
	// for example, if we cannot reach the Kubernetes control
	// plane.
	r.Equal("", res.Header().Get("X-KEDA-HTTP-Cold-Start"), "expected X-KEDA-HTTP-Cold-Start to be empty")

	// waitFunc should have been called, even though it timed out
	waitFuncCalled := false
	select {
	case <-waitFuncCalledCh:
		waitFuncCalled = true
	default:
	}

	r.True(waitFuncCalled, "wait function was not called")
}

// Test to make sure the proxy handler will wait for the waitFunc to
// complete
func TestWaitsForWaitFunc(t *testing.T) {
	r := require.New(t)

	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())

	waitFunc, waitFuncCalledCh, finishWaitFunc := notifyingFunc()
	const (
		noSuchHost     = "TestWaitsForWaitFunc.test"
		originRespCode = 201
		namespace      = "testns"
	)
	testSrv, testSrvURL, err := kedanet.StartTestServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(originRespCode)
		}),
	)
	r.NoError(err)
	defer testSrv.Close()
	originHost, originPort, err := splitHostPort(testSrvURL.Host)
	r.NoError(err)
	routingTable := routing.NewTable()
	routingTable.AddTarget(noSuchHost, routing.NewTarget(
		namespace,
		originHost,
		originPort,
		"nosuchdepl",
		1234,
	))
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		func(routing.Target) (*url.URL, error) {
			return testSrvURL, nil
		},
		forwardingConfig{
			waitTimeout:       timeouts.DeploymentReplicas,
			respHeaderTimeout: timeouts.ResponseHeader,
		},
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = noSuchHost

	// make the wait function finish after a short duration
	const waitDur = 100 * time.Millisecond
	go func() {
		time.Sleep(waitDur)
		finishWaitFunc()
	}()

	start := time.Now()
	hdl.ServeHTTP(res, req)
	elapsed := time.Since(start)
	r.NoError(waitForSignal(waitFuncCalledCh, 1*time.Second))

	// should take at least waitDur, but no more than waitDur*4
	r.GreaterOrEqual(elapsed, waitDur)
	r.Less(elapsed, waitDur*4)

	r.Equal(
		originRespCode,
		res.Code,
		"response code was unexpected",
	)
}

// the proxy should connect to a server, and then time out if
// the server doesn't respond in time
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
	waitFunc := func(context.Context, string, string) (int, error) {
		return 1, nil
	}
	routingTable := routing.NewTable()
	target := routing.NewTarget(
		"testns",
		"testsvc",
		9094,
		"testdepl",
		1234,
	)
	routingTable.AddTarget(originURL.Host, target)
	hdl := newForwardingHandler(
		logr.Discard(),
		routingTable,
		dialCtxFunc,
		waitFunc,
		func(routing.Target) (*url.URL, error) {
			return originURL, nil
		},
		forwardingConfig{
			waitTimeout:       timeouts.DeploymentReplicas,
			respHeaderTimeout: timeouts.ResponseHeader,
		},
	)
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req.Host = originURL.Host

	hdl.ServeHTTP(res, req)

	r.Equal("false", res.Header().Get("X-KEDA-HTTP-Cold-Start"), "expected X-KEDA-HTTP-Cold-Start false")
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
// before it returns).
//
// the _returned_ function won't itself return until the returned func()
// is called, or the context that is passed to it is done (e.g. cancelled, timed out,
// etc...). in the former case, the returned func itself returns nil. in the latter,
// it returns ctx.Err()
func notifyingFunc() (forwardWaitFunc, <-chan struct{}, func()) {
	calledCh := make(chan struct{})
	finishCh := make(chan struct{})
	finishFunc := func() {
		close(finishCh)
	}
	return func(ctx context.Context, _, _ string) (int, error) {
		close(calledCh)
		select {
		case <-finishCh:
			return 0, nil
		case <-ctx.Done():
			return 0, fmt.Errorf("TEST FUNCTION CONTEXT ERROR: %w", ctx.Err())
		}
	}, calledCh, finishFunc
}

func targetFromURL(
	ns string,
	u *url.URL,
	port int,
	deployment string,
	targetPendingReqs int32,
) routing.Target {
	svc := strings.Split(u.Host, ":")[0]
	return routing.NewTarget(
		ns,
		svc,
		port,
		deployment,
		targetPendingReqs,
	)
}
