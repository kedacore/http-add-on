package main

import (
	"net/http"
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
	t.Fatal("TODO")
}

// the proxy should connect to a server, and then time out if the server doesn't
// respond in time
func TestWaitHeaderTimeout(t *testing.T) {
	t.Fatal("TODO")
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

// ensureNoSignalAfter returns false is signalCh receives before timeout, true otherwise.
// it blocks for timeout at most
func ensureNoSignalBeforeTimeout(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-signalCh:
		return false
	}
}
