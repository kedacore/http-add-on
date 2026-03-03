package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	traceID              = "a8419b25ec2051e5"
	fullW3CLengthTraceID = "29b3290dc5a93f2618b17502ccb2a728"
	spanID               = "97337bce1bc3e368"
	parentSpanID         = "2890e7e08fc6592b"
	sampled              = "1"
	w3cPadding           = "0000000000000000"
)

func TestB3MultiPropagation(t *testing.T) {
	// Given
	r := require.New(t)

	microservice, microserviceURL, closeServer := startMicroservice(t)
	defer closeServer()

	exporter, tracerProvider := setupOTelSDKForTesting()
	instrumentedServeHTTP := withAutoInstrumentation(serveHTTP)

	request, responseWriter := createRequestAndResponse("GET", microserviceURL)

	request.Header.Set("X-B3-Traceid", traceID)
	request.Header.Set("X-B3-Spanid", spanID)
	request.Header.Set("X-B3-Parentspanid", parentSpanID)
	request.Header.Set("X-B3-Sampled", sampled)

	defer func(traceProvider *trace.TracerProvider, ctx context.Context) {
		_ = traceProvider.Shutdown(ctx)
	}(tracerProvider, request.Context())

	// When
	instrumentedServeHTTP.ServeHTTP(responseWriter, request)

	// Then
	receivedRequest := microservice.IncomingRequests()[0]
	receivedHeaders := receivedRequest.Header

	r.Equal(parentSpanID, receivedHeaders.Get("X-B3-Parentspanid"))
	r.Equal(traceID, receivedHeaders.Get("X-B3-Traceid"))
	r.Equal(spanID, receivedHeaders.Get("X-B3-Spanid"))
	r.Equal(sampled, receivedHeaders.Get("X-B3-Sampled"))

	r.NotContains(receivedHeaders, "Traceparent")
	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	if len(exportedSpans) != 1 {
		t.Fatalf("Expected 1 Span, got %d", len(exportedSpans))
	}
	sc := exportedSpans[0].SpanContext
	r.Equal(w3cPadding+traceID, sc.TraceID().String())
	r.NotEqual(spanID, sc.SpanID().String())
}

func TestW3CAndB3MultiPropagation(t *testing.T) {
	// Given
	r := require.New(t)

	microservice, microserviceURL, closeServer := startMicroservice(t)
	defer closeServer()

	exporter, tracerProvider := setupOTelSDKForTesting()
	instrumentedServeHTTP := withAutoInstrumentation(serveHTTP)

	request, responseWriter := createRequestAndResponse("GET", microserviceURL)

	request.Header.Set("X-B3-Traceid", traceID)
	request.Header.Set("X-B3-Spanid", spanID)
	request.Header.Set("X-B3-Parentspanid", parentSpanID)
	request.Header.Set("X-B3-Sampled", sampled)
	request.Header.Set("Traceparent", w3cPadding+traceID)

	defer func(traceProvider *trace.TracerProvider, ctx context.Context) {
		_ = traceProvider.Shutdown(ctx)
	}(tracerProvider, request.Context())

	// When
	instrumentedServeHTTP.ServeHTTP(responseWriter, request)

	// Then
	receivedRequest := microservice.IncomingRequests()[0]
	receivedHeaders := receivedRequest.Header

	r.Equal(parentSpanID, receivedHeaders.Get("X-B3-Parentspanid"))
	r.Equal(traceID, receivedHeaders.Get("X-B3-Traceid"))
	r.Equal(spanID, receivedHeaders.Get("X-B3-Spanid"))
	r.Equal(sampled, receivedHeaders.Get("X-B3-Sampled"))
	r.Equal(w3cPadding+traceID, receivedHeaders.Get("Traceparent"))

	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	if len(exportedSpans) != 1 {
		t.Fatalf("Expected 1 Span, got %d", len(exportedSpans))
	}
	sc := exportedSpans[0].SpanContext
	r.Equal(w3cPadding+traceID, sc.TraceID().String())
	r.NotEqual(spanID, sc.SpanID().String())
}

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

	r.Equal(receivedHeaders.Get("Traceparent"), traceParent)

	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")
	r.NotContains(receivedHeaders, "X-B3-Parentspanid")
	r.NotContains(receivedHeaders, "X-B3-Traceid")
	r.NotContains(receivedHeaders, "X-B3-Spanid")
	r.NotContains(receivedHeaders, "X-B3-Sampled")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	if len(exportedSpans) != 1 {
		t.Fatalf("Expected 1 Span, got %d", len(exportedSpans))
	}
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

	r.NotContains(receivedHeaders, "Traceparent")
	r.NotContains(receivedHeaders, "B3")
	r.NotContains(receivedHeaders, "b3")
	r.NotContains(receivedHeaders, "X-B3-Parentspanid")
	r.NotContains(receivedHeaders, "X-B3-Traceid")
	r.NotContains(receivedHeaders, "X-B3-Spanid")
	r.NotContains(receivedHeaders, "X-B3-Sampled")

	_ = tracerProvider.ForceFlush(request.Context())

	exportedSpans := exporter.GetSpans()
	if len(exportedSpans) != 1 {
		t.Fatalf("Expected 1 Span, got %d", len(exportedSpans))
	}
	sc := exportedSpans[0].SpanContext
	r.NotEmpty(sc.SpanID())
	r.NotEmpty(sc.TraceID())

	hasServiceAttribute := false
	hasColdStartAttribute := false
	for _, attribute := range exportedSpans[0].Attributes {
		if attribute.Key == "service" && attribute.Value.AsString() == "keda-http-interceptor-proxy-upstream" {
			hasServiceAttribute = true
		}

		if attribute.Key == "cold-start" {
			hasColdStartAttribute = true
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
	req = util.RequestWithStream(req, forwardURL)
	timeouts := defaultTimeouts()
	dialCtxFunc := retryDialContextFunc(timeouts)
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	uh := NewUpstream(rt, config.Tracing{}, false)
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
	timeouts.Connect = 1 * time.Millisecond
	timeouts.ResponseHeader = 1 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts)
	res, req, err := reqAndRes("/testfwd")
	r.NoError(err)
	req = util.RequestWithStream(req, originURL)
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	uh := NewUpstream(rt, config.Tracing{}, false)
	uh.ServeHTTP(res, req)

	forwardedRequests := hdl.IncomingRequests()
	r.Empty(forwardedRequests)
	r.Equal(502, res.Code)
	r.Contains(res.Body.String(), http.StatusText(http.StatusBadGateway))
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
		Connect:   originDelay,
		KeepAlive: 2 * time.Second,
		// the handler is going to take 500 milliseconds to respond, so make the
		// forwarder wait much longer than that
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
	req = util.RequestWithStream(req, originURL)
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	uh := NewUpstream(rt, config.Tracing{}, false)
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

	timeouts := defaultTimeouts()
	timeouts.DialRetryTimeout = 500 * time.Millisecond
	dialCtxFunc := retryDialContextFunc(timeouts)
	res, req, err := reqAndRes("/test")
	r.NoError(err)
	req = util.RequestWithStream(req, noSuchURL)
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	uh := NewUpstream(rt, config.Tracing{}, false)

	start := time.Now()
	uh.ServeHTTP(res, req)
	elapsed := time.Since(start)
	log.Printf("forwardRequest took %s", elapsed)

	r.GreaterOrEqualf(
		elapsed,
		timeouts.DialRetryTimeout,
		"proxy returned after %s, expected not to return until %s",
		elapsed,
		timeouts.DialRetryTimeout,
	)
	r.Equal(
		502,
		res.Code,
		"unexpected code (response body was '%s')",
		res.Body.String(),
	)
	r.Contains(res.Body.String(), http.StatusText(http.StatusBadGateway))
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
	req = util.RequestWithStream(req, srvURL)
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	uh := NewUpstream(rt, config.Tracing{}, false)
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
			upstream := NewUpstream(http.DefaultTransport, config.Tracing{}, false)

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
			req = util.RequestWithStream(req, backendURL)

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
		Connect:          100 * time.Millisecond,
		KeepAlive:        100 * time.Millisecond,
		ResponseHeader:   500 * time.Millisecond,
		WorkloadReplicas: 1 * time.Second,
		DialRetryTimeout: 1 * time.Second,
	}
}

func retryDialContextFunc(timeouts config.Timeouts) kedanet.DialContextFunc {
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	return kedanet.DialContextWithRetry(dialer, timeouts.DialRetryTimeout)
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
	rt := newRoundTripper(dialCtxFunc, timeouts.ResponseHeader)
	upstream := NewUpstream(rt, config.Tracing{Enabled: true}, false)

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
	ctx := util.ContextWithStream(context.Background(), url)
	request, _ := http.NewRequestWithContext(ctx, method, url.String(), nil)
	recorder := httptest.NewRecorder()
	return request, recorder
}

func withAutoInstrumentation(sut func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return otelhttp.NewHandler(http.HandlerFunc(sut), "SystemUnderTest")
}
