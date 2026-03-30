//go:build e2e

package helpers

import (
	"cmp"
	"crypto/tls"
	"io"
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

// WebSocketDial opens a WebSocket connection through the interceptor proxy.
// The connection is closed automatically via t.Cleanup.
func (f *Framework) WebSocketDial(host, path string) *websocket.Conn {
	f.t.Helper()

	wsURL := url.URL{Scheme: "ws", Host: f.proxyAddr, Path: path}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL.String(), http.Header{"Host": []string{host}})
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
