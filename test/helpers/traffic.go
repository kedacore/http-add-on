//go:build e2e

package helpers

import (
	"cmp"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Request struct {
	Method    string // defaults to GET
	Host      string
	Path      string
	Query     string // raw query string without leading "?"
	Headers   map[string]string
	Body      io.Reader
	TLSConfig *tls.Config
}

type HTTPResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Response struct {
	HTTPResponse
	Hostname       string
	RequestPath    string
	RequestHeaders http.Header
}

// ProxyRequest sends an HTTP request through the interceptor and parses the
// backend's response into structured fields.
func (f *Framework) ProxyRequest(r Request) Response {
	f.t.Helper()

	raw := f.ProxyRequestRaw(r)
	parsed := parseWhoami(raw.Body)

	return Response{
		HTTPResponse:   raw,
		Hostname:       parsed.Hostname,
		RequestPath:    parsed.RequestPath,
		RequestHeaders: parsed.RequestHeaders,
	}
}

// ProxyRequestRaw sends an HTTP request through the interceptor and returns
// the raw HTTP response without parsing the backend's output.
func (f *Framework) ProxyRequestRaw(r Request) HTTPResponse {
	f.t.Helper()

	reqURL := url.URL{Scheme: "http", Host: f.proxyAddr, Path: r.Path, RawQuery: r.Query}
	client := &http.Client{Timeout: 60 * time.Second}
	if r.TLSConfig != nil {
		reqURL.Scheme = "https"
		client.Transport = &http.Transport{TLSClientConfig: r.TLSConfig}
	}

	req, err := http.NewRequestWithContext(f.ctx, cmp.Or(r.Method, http.MethodGet), reqURL.String(), r.Body)
	if err != nil {
		f.t.Fatalf("failed to create request: %v", err)
	}

	if r.Host != "" {
		req.Host = r.Host
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		f.t.Fatalf("HTTP %s %s%s failed: %v", req.Method, r.Host, r.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatalf("failed to read response body: %v", err)
	}

	return HTTPResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
	}
}

// AssertRouteReachesApp sends a request through the proxy and verifies it
// returns 200 and is served by the expected backend app. When the request
// includes a path, the backend's reported request path is also verified.
func (f *Framework) AssertRouteReachesApp(r Request, expectedApp string) {
	f.t.Helper()

	resp := f.ProxyRequest(r)
	if resp.StatusCode != http.StatusOK {
		f.t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, string(resp.Body))
	}
	if resp.Hostname != expectedApp {
		f.t.Fatalf("expected request to be served by %s, got %q; body: %s", expectedApp, resp.Hostname, string(resp.Body))
	}
	if r.Path != "" && resp.RequestPath != r.Path {
		f.t.Fatalf("expected request path %s, got %q", r.Path, resp.RequestPath)
	}
}

// AssertRouteRejected sends a request through the proxy and verifies it is
// NOT served successfully (non-200 status code). Fails the test if the
// request unexpectedly succeeds.
func (f *Framework) AssertRouteRejected(r Request) {
	f.t.Helper()

	resp := f.ProxyRequestRaw(r)
	if resp.StatusCode == http.StatusOK {
		f.t.Fatalf("expected non-200 status, got 200; body: %s", string(resp.Body))
	}
}

// AssertStatus sends a request through the proxy and verifies it returns the
// expected HTTP status code.
func (f *Framework) AssertStatus(r Request, expectedStatus int) {
	f.t.Helper()

	resp := f.ProxyRequestRaw(r)
	if resp.StatusCode != expectedStatus {
		f.t.Fatalf("expected status %d, got %d; body: %s", expectedStatus, resp.StatusCode, string(resp.Body))
	}
}

// WebSocketDial opens a WebSocket connection through the interceptor proxy.
// The connection is closed automatically via t.Cleanup.
func (f *Framework) WebSocketDial(host, path string) *websocket.Conn {
	f.t.Helper()

	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		NetDialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	}

	wsURL := url.URL{Scheme: "ws", Host: f.proxyAddr, Path: path}
	conn, resp, err := dialer.Dial(wsURL.String(), http.Header{"Host": []string{host}})
	if err != nil {
		f.t.Fatalf("WebSocket dial to %s%s failed: %v", host, path, err)
	}
	_ = resp.Body.Close()
	f.t.Cleanup(func() { _ = conn.Close() })

	return conn
}

type whoamiResult struct {
	Hostname       string
	RequestPath    string
	RequestHeaders http.Header
}

// parseWhoami extracts fields from the whoami default text response:
//
//	Hostname: app-a-7b6dc9f6bf-lcq26
//	...
//	GET /path HTTP/1.1
//	Host: example.com
//	X-Forwarded-For: 10.0.0.1
//	...
func parseWhoami(body []byte) whoamiResult {
	r := whoamiResult{RequestHeaders: http.Header{}}
	pastRequestLine := false
	for line := range strings.SplitSeq(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "Hostname: "):
			r.Hostname = strings.TrimPrefix(line, "Hostname: ")
		case !pastRequestLine:
			if parts := strings.Fields(line); len(parts) == 3 && strings.HasPrefix(parts[2], "HTTP/") {
				r.RequestPath = parts[1]
				pastRequestLine = true
			}
		default:
			if k, v, ok := strings.Cut(line, ": "); ok {
				r.RequestHeaders.Add(k, v)
			}
		}
	}
	return r
}
