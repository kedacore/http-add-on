package main

import (
	"log"
	"math"
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
		}
	}
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	return kedanet.DialContextWithRetry(dialer), timeouts
}

func sumExp(initial, num int) int {
	ret := initial
	for i := 2; i <= num; i++ {
		ret += int(math.Pow(float64(initial), float64(num)))
	}
	return ret
}

func reqAndRes(path string) (*httptest.ResponseRecorder, *http.Request, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}
	resRecorder := httptest.NewRecorder()
	return resRecorder, req, nil
}

func TestForwarderSuccess(t *testing.T) {
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
		ensureSignalBeforeTimeout(reqRecvCh, 100*time.Millisecond),
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
func TestForwarderHeaderTimeout(t *testing.T) {
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
	r.Contains(res.Body.String(), "Error on backend")
	// the proxy has bailed out, so tell the origin to stop
	close(originWaitCh)
}

// Test to ensure that the request forwarder waits for an origin that is slow
func TestForwarderWaitsForSlowOrigin(t *testing.T) {
	r := require.New(t)
	// the origin will wait until this channel receives or is closed
	originWaitCh := make(chan struct{})
	const originRespCode = 200
	const originRespBodyStr = "Hello World!"
	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		<-originWaitCh
		w.WriteHeader(originRespCode)
		w.Write([]byte(originRespBodyStr))
	})
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	// the origin is gonna wait this long, and we'll make the proxy
	// have a much longer timeout than this to account for timing issues
	const originDelay = 500 * time.Millisecond
	timeouts := &config.Timeouts{
		Connect:   500 * time.Millisecond,
		KeepAlive: 2 * time.Second,
		// the handler is going to take 500 milliseconds to respond, so make the
		// forwarder wait much longer than that
		ResponseHeader: originDelay * 4,
	}
	dialCtxFunc, timeouts := retryDialContextFunc(timeouts)
	go func() {
		// wait for 100ms less than
		time.Sleep(originDelay)
		close(originWaitCh)
	}()
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	forwardRequest(
		res,
		req,
		dialCtxFunc,
		timeouts.ResponseHeader,
		originURL,
	)
	// wait for the goroutine above to finish, with a little cusion
	ensureSignalBeforeTimeout(originWaitCh, originDelay*2)
	r.Equal(originRespCode, res.Code)
	r.Equal(originRespBodyStr, res.Body.String())

}

func TestForwarderConnectionRetryAndTimeout(t *testing.T) {
	r := require.New(t)
	noSuchURL, err := url.Parse("https://localhost:65533")
	r.NoError(err)

	timeouts := &config.Timeouts{
		Connect:        2 * time.Millisecond,
		KeepAlive:      1 * time.Millisecond,
		ResponseHeader: 50 * time.Millisecond,
	}
	dialCtxFunc, timeouts := retryDialContextFunc(timeouts)
	res, req, err := reqAndRes("/test")
	r.NoError(err)

	// this channel will be closed after forwardRequest returns
	start := time.Now()
	forwardDoneSignal := make(chan struct{})
	go func() {
		start := time.Now()
		forwardRequest(
			res,
			req,
			dialCtxFunc,
			timeouts.ResponseHeader,
			noSuchURL,
		)
		log.Printf("forwardRequest took %s", time.Since(start))
		close(forwardDoneSignal)
	}()

	// forwardDoneSignal should close _after_ the total timeout of forwardRequest.
	//
	// forwardRequest uses dialCtxFunc to establish network connections, and dialCtxFunc does
	// exponential backoff. It starts at 2ms (timeouts.Connect above), doubles every time, and stops after 5 tries,
	// so that's 2ms + 4ms + 8ms + 16ms + 32ms, or SUM(2^N) where N is in [1, 5]
	expectedForwardTimeout := time.Duration(sumExp(2, 5)) * time.Millisecond
	r.True(
		ensureNoSignalBeforeTimeout(forwardDoneSignal, expectedForwardTimeout),
		"signal returned after %s, expected not to return until %s",
		time.Since(start),
		expectedForwardTimeout,
	)
	r.Equal(
		502,
		res.Code,
		"unexpected code, response body was '%s'",
		res.Body.String(),
	)
	r.Contains(res.Body.String(), "Error on backend")
}
