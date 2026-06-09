package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	fullW3CLengthTraceID = "29b3290dc5a93f2618b17502ccb2a728"
	spanID               = "97337bce1bc3e368"
)

func TestW3CPropagation(t *testing.T) {
	// Given
	r := require.New(t)

	microservice, microserviceURL, closeServer := startMicroservice(t)
	defer closeServer()

	exporter, tracerProvider := setupOTelSDKForTesting()
	instrumentedServeHTTP := withAutoInstrumentation(serveHTTP)

	request, responseWriter := createRequestAndResponse("GET", microserviceURL)

	traceParent := fmt.Sprintf("00-%s-%s-01", fullW3CLengthTraceID, spanID)
	request.Header.Set("Traceparent", traceParent)

	defer func(traceProvider *trace.TracerProvider, ctx context.Context) {
		_ = traceProvider.Shutdown(ctx)
	}(tracerProvider, request.Context())

	// When
	instrumentedServeHTTP.ServeHTTP(responseWriter, request)

	// Then
	receivedRequest := microservice.IncomingRequests()[0]
	receivedHeaders := receivedRequest.Header

	r.Contains(receivedHeaders.Get("Traceparent"), fullW3CLengthTraceID)

	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")
	r.NotContains(receivedHeaders, "X-B3-Parentspanid")
	r.NotContains(receivedHeaders, "X-B3-Traceid")
	r.NotContains(receivedHeaders, "X-B3-Spanid")
	r.NotContains(receivedHeaders, "X-B3-Sampled")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	r.GreaterOrEqual(len(exportedSpans), 1, "expected at least 1 span")
	sc := exportedSpans[0].SpanContext
	r.Equal(fullW3CLengthTraceID, sc.TraceID().String())
	r.True(sc.IsSampled())
	r.NotEqual(spanID, sc.SpanID().String())
}

func TestPropagationWhenNoHeaders(t *testing.T) {
	// Given
	r := require.New(t)

	microservice, microserviceURL, closeServer := startMicroservice(t)
	defer closeServer()

	exporter, tracerProvider := setupOTelSDKForTesting()
	instrumentedServeHTTP := withAutoInstrumentation(serveHTTP)

	request, responseWriter := createRequestAndResponse("GET", microserviceURL)

	defer func(traceProvider *trace.TracerProvider, ctx context.Context) {
		_ = traceProvider.Shutdown(ctx)
	}(tracerProvider, request.Context())

	// When
	instrumentedServeHTTP.ServeHTTP(responseWriter, request)

	// Then
	receivedRequest := microservice.IncomingRequests()[0]
	receivedHeaders := receivedRequest.Header

	r.Contains(receivedHeaders, "Traceparent")
	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")
	r.NotContains(receivedHeaders, "X-B3-Parentspanid")
	r.NotContains(receivedHeaders, "X-B3-Traceid")
	r.NotContains(receivedHeaders, "X-B3-Spanid")
	r.NotContains(receivedHeaders, "X-B3-Sampled")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	r.GreaterOrEqual(len(exportedSpans), 1, "expected at least 1 span")
	sc := exportedSpans[0].SpanContext
	r.NotEmpty(sc.SpanID())
	r.NotEmpty(sc.TraceID())

	hasServiceAttribute := false
	hasColdStartAttribute := false
	for _, s := range exportedSpans {
		for _, attribute := range s.Attributes {
			if attribute.Key == "service" && attribute.Value.AsString() == "keda-http-interceptor-proxy-upstream" {
				hasServiceAttribute = true
			}
			if attribute.Key == "cold-start" {
				hasColdStartAttribute = true
			}
		}
	}
	r.True(hasServiceAttribute)
	r.True(hasColdStartAttribute)
}

func TestForwarderSuccess(t *testing.T) {
	r := require.New(t)
	// this channel will be closed after the request was received, but
	// before the response was sent
	reqRecvCh := make(chan struct{})
	const respCode = 302
	const respBody = "TestForwardingHandler"
	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			close(reqRecvCh)
			w.WriteHeader(respCode)
			_, err := w.Write([]byte(respBody))
			assert.NoError(t, err)
		}),
	)
	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req = util.RequestWithUpstreamURL(req, forwardURL)
	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts)
	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)
	uh.ServeHTTP(res, req)

	r.True(
		ensureSignalBeforeTimeout(reqRecvCh, 100*time.Millisecond),
		"request was not received within %s",
		100*time.Millisecond,
	)
	forwardedRequests := originHdl.IncomingRequests()
	r.Len(forwardedRequests, 1, "number of requests forwarded")
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
	timeouts.ResponseHeader = 5 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	req = util.RequestWithUpstreamURL(req, originURL)
	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)
	uh.ServeHTTP(res, req)

	r.Equal(http.StatusGatewayTimeout, res.Code)
	r.Contains(res.Body.String(), http.StatusText(http.StatusGatewayTimeout))
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
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			<-originWaitCh
			w.WriteHeader(originRespCode)
			_, err := w.Write([]byte(originRespBodyStr))
			assert.NoError(t, err)
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	// the origin is gonna wait this long, and we'll make the proxy
	// have a much longer timeout than this to account for timing issues
	const originDelay = 5 * time.Millisecond
	timeouts := config.Timeouts{
		Connect:        originDelay,
		ResponseHeader: originDelay * 4,
	}

	dialCtxFunc := retryDialContextFunc(timeouts)
	go func() {
		time.Sleep(originDelay)
		close(originWaitCh)
	}()
	const path = "/testfwd"
	res, req, err := reqAndRes(path)
	r.NoError(err)
	req = util.RequestWithUpstreamURL(req, originURL)
	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)
	uh.ServeHTTP(res, req)
	// wait for the goroutine above to finish, with a little cusion
	ensureSignalBeforeTimeout(originWaitCh, originDelay*2)
	r.Equal(originRespCode, res.Code)
	r.Equal(originRespBodyStr, res.Body.String())
}

func TestForwarderConnectionRetryAndTimeout(t *testing.T) {
	r := require.New(t)
	noSuchURL, err := url.Parse("https://localhost:65533")
	r.NoError(err)

	const requestTimeout = 500 * time.Millisecond
	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts)
	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)

	res, req, err := reqAndRes("/test")
	r.NoError(err)

	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	req = util.RequestWithUpstreamURL(req, noSuchURL)

	start := time.Now()
	uh.ServeHTTP(res, req)
	elapsed := time.Since(start)
	log.Printf("forwardRequest took %s", elapsed)

	r.GreaterOrEqualf(
		elapsed,
		requestTimeout,
		"proxy returned after %s, expected not to return until %s",
		elapsed,
		requestTimeout,
	)
	r.Equal(
		http.StatusGatewayTimeout,
		res.Code,
		"unexpected code (response body was '%s')",
		res.Body.String(),
	)
	r.Contains(res.Body.String(), http.StatusText(http.StatusGatewayTimeout))
}

func TestForwardRequestRedirectAndHeaders(t *testing.T) {
	r := require.New(t)

	srv, srvURL, err := kedanet.StartTestServer(
		kedanet.NewTestHTTPHandlerWrapper(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("X-Custom-Header", "somethingcustom")
				w.Header().Set("Location", "abc123.com")
				w.WriteHeader(301)
				_, err := w.Write([]byte("Hello from srv"))
				assert.NoError(t, err)
			}),
		),
	)
	r.NoError(err)
	defer srv.Close()

	timeouts := defaultTimeouts()
	timeouts.Connect = 10 * time.Millisecond
	timeouts.ResponseHeader = 10 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	req = util.RequestWithUpstreamURL(req, srvURL)
	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)
	uh.ServeHTTP(res, req)
	r.Equal(301, res.Code)
	r.Equal("abc123.com", res.Header().Get("Location"))
	r.Equal("text/html; charset=utf-8", res.Header().Get("Content-Type"))
	r.Equal("somethingcustom", res.Header().Get("X-Custom-Header"))
	r.Equal("Hello from srv", res.Body.String())
}

func TestUpstreamPreservesXForwardedHeaders(t *testing.T) {
	tests := map[string]struct {
		forwardedFor   string
		forwardedHost  string
		forwardedProto string
		forwardedPort  string
	}{
		"preserves and extends forwarded IPs": {
			forwardedFor: "198.51.100.1",
		},
		"preserves forwarded host": {
			forwardedHost: "example.org",
		},
		"preserves forwarded proto": {
			forwardedProto: "http",
		},
		"preserves forwarded port": {
			forwardedPort: "443",
		},
		"preserves and extends existing headers": {
			forwardedFor:   "1.2.3.4, 5.6.7.8",
			forwardedHost:  "keda.sh",
			forwardedProto: "https",
			forwardedPort:  "8443",
		},
		"sets header when not present": {
			forwardedFor:   "",
			forwardedHost:  "",
			forwardedProto: "",
			forwardedPort:  "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Prepare a fake backend
			var receivedHeaders http.Header
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header.Clone()
			}))
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			if err != nil {
				t.Fatalf("failed to parse backend URL: %v", err)
			}

			// Configure the Upstream and send a dummy request
			upstream := NewUpstream(http.DefaultTransport.(*http.Transport), newFakeClient(), config.Tracing{}, 500*time.Millisecond)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			if tt.forwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.forwardedPort != "" {
				req.Header.Set("X-Forwarded-Port", tt.forwardedPort)
			}
			req = util.RequestWithUpstreamURL(req, backendURL)

			upstream.ServeHTTP(httptest.NewRecorder(), req)

			// Verify the test conditions
			xff := receivedHeaders.Get("X-Forwarded-For")
			if tt.forwardedFor != "" {
				if !strings.HasPrefix(xff, tt.forwardedFor+", ") {
					t.Errorf("expected X-Forwarded-For to start with %q, got: %q", tt.forwardedFor+", ", xff)
				}
			} else if xff == "" {
				t.Error("X-Forwarded-For should contain at least the client IP")
			}

			xfh := receivedHeaders.Get("X-Forwarded-Host")
			if tt.forwardedHost != "" {
				if tt.forwardedHost != xfh {
					t.Errorf("expected forwarded host %q, got %q", tt.forwardedHost, xfh)
				}
			} else if xfh != req.Host {
				t.Errorf("expected default forwarded host %q, got %q", req.Host, xfh)
			}

			xfproto := receivedHeaders.Get("X-Forwarded-Proto")
			if tt.forwardedProto != "" {
				if tt.forwardedProto != xfproto {
					t.Errorf("expected forwarded proto %q, got %q", tt.forwardedProto, xfproto)
				}
			} else if xfproto != "http" {
				t.Errorf("expected default forwarded proto %q, got %q", "http", xfproto)
			}

			// Ensure that X-Forwarded-Port is preserved even if we don't set a default for it
			xfport := receivedHeaders.Get("X-Forwarded-Port")
			if xfport != tt.forwardedPort {
				t.Errorf("expected forwarded port %q, got %q", tt.forwardedPort, xfport)
			}
		})
	}
}

func TestUpstream_RouteSpecResponseHeaderOverride(t *testing.T) {
	r := require.New(t)
	originWaitCh := make(chan struct{})
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			<-originWaitCh
			w.WriteHeader(200)
		}),
	)
	srv, originURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	// Global timeout is generous, but per-route override is very tight
	timeouts := defaultTimeouts()
	timeouts.ResponseHeader = 5 * time.Second
	dialCtxFunc := retryDialContextFunc(timeouts)

	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	req = util.RequestWithUpstreamURL(req, originURL)

	// Set IR with tight per-route ResponseHeader override
	ir := &httpv1beta1.InterceptorRoute{
		Spec: httpv1beta1.InterceptorRouteSpec{
			Timeouts: httpv1beta1.InterceptorRouteTimeouts{
				ResponseHeader: &metav1.Duration{Duration: 1 * time.Millisecond},
			},
		},
	}
	ctx := util.ContextWithInterceptorRoute(req.Context(), ir)
	req = req.WithContext(ctx)

	uh := NewUpstream(newTestTransport(dialCtxFunc), newFakeClient(), config.Tracing{}, timeouts.ResponseHeader)
	uh.ServeHTTP(res, req)

	r.Equal(http.StatusGatewayTimeout, res.Code)
	close(originWaitCh)
}

// TestFullDuplexBodyPanic verifies that the upstream handler does not trigger
// a "invalid concurrent Body.Read call" panic (golang/go#68560) when the
// reverse proxy's RoundTrip fails without consuming the request body.
//
// The panic is recovered by the server and logged via its ErrorLog.
func TestFullDuplexBodyPanic(t *testing.T) {
	failingTransport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
	}

	upstream := NewUpstream(failingTransport, newFakeClient(), config.Tracing{}, 1*time.Second)

	targetURL, _ := url.Parse("http://fake-backend:8080")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(util.ContextWithUpstreamURL(r.Context(), targetURL))
		upstream.ServeHTTP(w, r)
	})

	var panicked atomic.Bool
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ErrorLog = log.New(panicLogWriter{onPanic: func() { panicked.Store(true) }}, "", 0)
	srv.Start()
	defer srv.Close()

	resp, err := srv.Client().Post(srv.URL+"/test", "application/json", strings.NewReader(`{"key":"value"}`))
	if err == nil {
		_ = resp.Body.Close()
	}

	// The panic fires asynchronously after the response is sent.
	time.Sleep(5 * time.Millisecond)

	if panicked.Load() {
		t.Fatal("server panicked with 'invalid concurrent Body.Read call' (golang/go#68560)")
	}
}

// panicLogWriter detects panics logged by net/http's server error recovery.
type panicLogWriter struct{ onPanic func() }

func (w panicLogWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "panic") {
		w.onPanic()
	}
	return len(p), nil
}

func TestUpstream_AppProtocolTransportSelection(t *testing.T) {
	h2cProto := "kubernetes.io/h2c"

	tests := map[string]struct {
		appProtocol        *string
		incomingProtoMajor int
		wantBackendProto   int
	}{
		"h2c appProtocol with HTTP/1 request uses HTTP/2": {
			appProtocol:        &h2cProto,
			incomingProtoMajor: 1,
			wantBackendProto:   2,
		},
		"h2c appProtocol with HTTP/2 request uses HTTP/2": {
			appProtocol:        &h2cProto,
			incomingProtoMajor: 2,
			wantBackendProto:   2,
		},
		"no appProtocol with HTTP/1 request uses HTTP/1": {
			appProtocol:        nil,
			incomingProtoMajor: 1,
			wantBackendProto:   1,
		},
		"no appProtocol with HTTP/2 request uses HTTP/1": {
			appProtocol:        nil,
			incomingProtoMajor: 2,
			wantBackendProto:   1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var backendProtoMajor int
			backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendProtoMajor = r.ProtoMajor
				w.WriteHeader(http.StatusOK)
			}))
			var protocols http.Protocols
			protocols.SetHTTP1(true)
			protocols.SetUnencryptedHTTP2(true)
			backend.Config.Protocols = &protocols
			backend.Start()
			defer backend.Close()

			backendURL, err := url.Parse(backend.URL)
			if err != nil {
				t.Fatalf("failed to parse backend URL: %v", err)
			}

			fakeClient := newFakeClient(corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{
						Name:        "grpc",
						Port:        8080,
						AppProtocol: tc.appProtocol,
					}},
				},
			})

			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ir",
					Namespace: "default",
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Service: "test-svc",
						Port:    8080,
					},
				},
			}

			upstream := NewUpstream(http.DefaultTransport.(*http.Transport), fakeClient, config.Tracing{}, 5*time.Second)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.ProtoMajor = tc.incomingProtoMajor
			ctx := util.ContextWithUpstreamURL(req.Context(), backendURL)
			ctx = util.ContextWithInterceptorRoute(ctx, ir)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			upstream.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if backendProtoMajor != tc.wantBackendProto {
				t.Errorf("backend saw HTTP/%d, want HTTP/%d", backendProtoMajor, tc.wantBackendProto)
			}
		})
	}
}

func TestUpstream_AppProtocolFallbackOnMissingService(t *testing.T) {
	var backendProtoMajor int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendProtoMajor = r.ProtoMajor
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("failed to parse backend URL: %v", err)
	}

	fakeClient := newFakeClient()

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ir",
			Namespace: "default",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "missing-svc",
				Port:    8080,
			},
		},
	}

	upstream := NewUpstream(http.DefaultTransport.(*http.Transport), fakeClient, config.Tracing{}, 5*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := util.ContextWithUpstreamURL(req.Context(), backendURL)
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	upstream.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if backendProtoMajor != 1 {
		t.Errorf("expected fallback to HTTP/1, got HTTP/%d", backendProtoMajor)
	}
}

func newFakeClient(objs ...corev1.Service) client.Reader {
	builder := fake.NewClientBuilder().WithScheme(cache.NewScheme())
	for i := range objs {
		builder = builder.WithObjects(&objs[i])
	}
	return builder.Build()
}

func newTestTransport(dialCtxFunc kedanet.DialContextFunc) *http.Transport {
	return &http.Transport{
		DialContext: dialCtxFunc,
	}
}

func defaultTimeouts() config.Timeouts {
	return config.Timeouts{
		Connect:        100 * time.Millisecond,
		Readiness:      1 * time.Second,
		Request:        10 * time.Second,
		ResponseHeader: 500 * time.Millisecond,
	}
}

func retryDialContextFunc(timeouts config.Timeouts) kedanet.DialContextFunc {
	return kedanet.DialContextWithRetry(timeouts.Connect)
}

func reqAndRes(path string) (*httptest.ResponseRecorder, *http.Request, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, nil, err
	}
	resRecorder := httptest.NewRecorder()
	return resRecorder, req, nil
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

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts)
	transport := newTestTransport(dialCtxFunc)
	upstream := NewUpstream(transport, newFakeClient(), config.Tracing{Enabled: true}, timeouts.ResponseHeader)

	upstream.ServeHTTP(w, r)
}

func setupOTelSDKForTesting() (*tracetest.InMemoryExporter, *trace.TracerProvider) {
	exporter := tracetest.NewInMemoryExporter()
	traceProvider := trace.NewTracerProvider(trace.WithBatcher(exporter, trace.WithBatchTimeout(time.Second)))
	otel.SetTracerProvider(traceProvider)
	prop := tracing.NewPropagator()
	otel.SetTextMapPropagator(prop)
	return exporter, traceProvider
}

func startMicroservice(t *testing.T) (*kedanet.TestHTTPHandlerWrapper, *url.URL, func()) {
	r := require.New(t)
	requestReceiveChannel := make(chan struct{})

	const respCode = 200
	const respBody = "Success Response"
	microservice := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			close(requestReceiveChannel)
			w.WriteHeader(respCode)
			_, err := w.Write([]byte(respBody))
			assert.NoError(t, err)
		}),
	)
	server := httptest.NewServer(microservice)

	url, err := url.Parse(server.URL)
	r.NoError(err)

	return microservice, url, func() {
		server.Close()
	}
}

func createRequestAndResponse(method string, url *url.URL) (*http.Request, http.ResponseWriter) {
	ctx := util.ContextWithUpstreamURL(context.Background(), url)
	request, _ := http.NewRequestWithContext(ctx, method, url.String(), nil)
	recorder := httptest.NewRecorder()
	return request, recorder
}

func withAutoInstrumentation(sut func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(sut), "SystemUnderTest")
}
