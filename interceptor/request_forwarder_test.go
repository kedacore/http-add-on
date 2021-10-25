package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/interceptor/config"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func newRoundTripper(
	dialCtxFunc kedanet.DialContextFunc,
	httpRespHeaderTimeout time.Duration,
) http.RoundTripper {
	return &http.Transport{
		DialContext:           dialCtxFunc,
		ResponseHeaderTimeout: httpRespHeaderTimeout,
	}
}

func defaultTimeouts() config.Timeouts {
	return config.Timeouts{
		Connect:            100 * time.Millisecond,
		KeepAlive:          100 * time.Millisecond,
		ResponseHeader:     500 * time.Millisecond,
		DeploymentReplicas: 1 * time.Second,
	}
}

// returns a kedanet.DialContextFunc by calling kedanet.DialContextWithRetry. if you pass nil for the
// timeoutConfig, it uses standard values. otherwise it uses the one you passed.
//
// the returned config.Timeouts is what was passed to the DialContextWithRetry function
func retryDialContextFunc(
	timeouts config.Timeouts,
	backoff wait.Backoff,
) kedanet.DialContextFunc {
	dialer := kedanet.NewNetDialer(
		timeouts.Connect,
		timeouts.KeepAlive,
	)
	return kedanet.DialContextWithRetry(dialer, backoff)
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
	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(reqRecvCh)
			w.WriteHeader(respCode)
			w.Write([]byte(respBody))
		}),
	)
	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
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
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-originWaitCh
			w.WriteHeader(200)
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	timeouts.Connect = 10 * time.Millisecond
	timeouts.ResponseHeader = 10 * time.Millisecond
	backoff := timeouts.Backoff(2, 2, 1)
	dialCtxFunc := retryDialContextFunc(timeouts, backoff)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
		originURL,
	)

	forwardedRequests := hdl.IncomingRequests()
	r.Equal(0, len(forwardedRequests))
	r.Equal(502, res.Code)
	r.Contains(res.Body.String(), "error on backend")
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
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-originWaitCh
			w.WriteHeader(originRespCode)
			w.Write([]byte(originRespBodyStr))
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	// the origin is gonna wait this long, and we'll make the proxy
	// have a much longer timeout than this to account for timing issues
	const originDelay = 5 * time.Millisecond
	timeouts := config.Timeouts{
		Connect:   originDelay,
		KeepAlive: 2 * time.Second,
		// the handler is going to take 500 milliseconds to respond, so make the
		// forwarder wait much longer than that
		ResponseHeader: originDelay * 4,
	}

	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	go func() {
		time.Sleep(originDelay)
		close(originWaitCh)
	}()
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
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

	timeouts := config.Timeouts{
		Connect:        10 * time.Millisecond,
		KeepAlive:      1 * time.Millisecond,
		ResponseHeader: 50 * time.Millisecond,
	}
	dialCtxFunc := retryDialContextFunc(timeouts, timeouts.DefaultBackoff())
	res, req, err := reqAndRes("/test")
	r.NoError(err)

	start := time.Now()
	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
		noSuchURL,
	)
	elapsed := time.Since(start)
	log.Printf("forwardRequest took %s", elapsed)

	// forwardDoneSignal should close _after_ the total timeout of forwardRequest.
	//
	// forwardRequest uses dialCtxFunc to establish network connections, and dialCtxFunc does
	// exponential backoff. It starts at 2ms (timeouts.Connect above), doubles every time, and stops after 5 tries,
	// so that's 2ms + 4ms + 8ms + 16ms + 32ms, or SUM(2^N) where N is in [1, 5]
	expectedForwardTimeout := kedanet.MinTotalBackoffDuration(timeouts.DefaultBackoff())
	r.GreaterOrEqualf(
		elapsed,
		expectedForwardTimeout,
		"proxy returned after %s, expected not to return until %s",
		time.Since(start),
		expectedForwardTimeout,
	)
	r.Equal(
		502,
		res.Code,
		"unexpected code (response body was '%s')",
		res.Body.String(),
	)
	r.Contains(res.Body.String(), "error on backend")
}

func TestForwardRequestRedirectAndHeaders(t *testing.T) {
	r := require.New(t)

	srv, srvURL, err := kedanet.StartTestServer(
		kedanet.NewTestHTTPHandlerWrapper(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("X-Custom-Header", "somethingcustom")
				w.Header().Set("Location", "abc123.com")
				w.WriteHeader(301)
				w.Write([]byte("Hello from srv"))
			}),
		),
	)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	timeouts.Connect = 10 * time.Millisecond
	timeouts.ResponseHeader = 10 * time.Millisecond
	backoff := timeouts.Backoff(2, 2, 1)
	dialCtxFunc := retryDialContextFunc(timeouts, backoff)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
		srvURL,
	)
	r.Equal(301, res.Code)
	r.Equal("abc123.com", res.Header().Get("Location"))
	r.Equal("text/html; charset=utf-8", res.Header().Get("Content-Type"))
	r.Equal("somethingcustom", res.Header().Get("X-Custom-Header"))
	r.Equal("Hello from srv", res.Body.String())
}
