package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
)

func TestForwardRequestRedirectAndHeaders(t *testing.T) {
	r := require.New(t)
	hdrs := map[string]string{
		"Content-Type":    "text/html; charset=utf-8",
		"X-Custom-Header": "somethingcustom",
		"Location":        "abc123.com/def",
	}
	hdl := standardHandler(301, "hello", hdrs)
	// issue the request with a host.
	// expect that host to not be set on the
	// response header Location header. the host on
	// that header should instead be what's listed
	// on hdrs["Location"]
	const host = "myhost.com"
	const path = "/abc"
	res, _, err := issueRequest(hdl, host, path)
	r.NoError(err)
	r.Equal(301, res.Code)
	r.Equal("hello", res.Body.String())
	for k, v := range hdrs {
		// ensure that the all response headers show up exactly
		// as they were returned from the origin (hdl)
		r.Equal(v, res.Header().Get(k))
	}
}

func TestForwardRequestPathOnlyRedirectAndHeaders(t *testing.T) {
	r := require.New(t)
	hdrs := map[string]string{
		"Content-Type":    "text/html; charset=utf-8",
		"X-Custom-Header": "somethingcustom",
		"Location":        "/xyz",
	}
	hdl := standardHandler(301, "hello", hdrs)
	// issue the request with a host.
	// expect that host to not be set on the
	// response header Location header. the host on
	// that header should instead be what's listed
	// on hdrs["Location"]
	const host = "myhost.com"
	const path = "/abc"
	res, _, err := issueRequest(hdl, host, path)
	r.NoError(err)
	r.Equal(301, res.Code)
	r.Equal("hello", res.Body.String())
	for k, v := range hdrs {
		if k == "Location" {
			// the "Location" header should be the
			// path returned by the origin (hdl)
			// but it should also have the host
			// that was passed to issueRequest
			expected := fmt.Sprintf(
				"%s%s",
				host,
				v,
			)
			r.Equal(expected, res.Header().Get(k))
		} else {
			// ensure that all response headers except for
			// the "Location" header to show up exactly
			// as they were returned from the origin (hdl)
			r.Equal(v, res.Header().Get(k))
		}
	}
}

func standardHandler(
	code int,
	body string,
	headers map[string]string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		w.Write([]byte(body))
	}
}

func issueRequest(
	hdl http.HandlerFunc,
	inHost,
	inPath string,
) (*httptest.ResponseRecorder, *http.Request, error) {
	srv, srvURL, err := kedanet.StartTestServer(
		kedanet.NewTestHTTPHandlerWrapper(
			http.HandlerFunc(hdl),
		),
	)
	if err != nil {
		return nil, nil, err
	}
	defer srv.Close()

	timeouts := defaultTimeouts()
	timeouts.Connect = 10 * time.Millisecond
	timeouts.ResponseHeader = 10 * time.Millisecond
	backoff := timeouts.Backoff(2, 2, 1)
	dialCtxFunc := retryDialContextFunc(timeouts, backoff)

	res, req, err := reqAndRes(inPath)
	if err != nil {
		return nil, nil, err
	}
	req.Host = inHost

	forwardRequest(
		res,
		req,
		newRoundTripper(dialCtxFunc, timeouts.ResponseHeader),
		srvURL,
	)
	return res, req, nil

}
