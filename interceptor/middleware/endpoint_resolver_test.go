package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	testNamespace = "test-namespace"
	testService   = "testservice"
)

func TestEndpointResolver_ImmediatelyReady(t *testing.T) {
	tests := map[string]struct {
		enableColdStartHeader bool
		wantColdStartHeader   string
	}{
		"with cold-start header": {
			enableColdStartHeader: true,
			wantColdStartHeader:   "false",
		},
		"without cold-start header": {
			enableColdStartHeader: false,
			wantColdStartHeader:   "",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cache := k8s.NewReadyEndpointsCache(logr.Discard())
			addReadyEndpoint(cache)

			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
				ReadinessTimeout:      5 * time.Second,
				EnableColdStartHeader: tt.enableColdStartHeader,
			})

			rec := httptest.NewRecorder()
			req := newRequest(t, defaultIR())
			mw.ServeHTTP(rec, req)

			if !nextCalled {
				t.Fatal("expected next handler to be called")
			}
			if got, want := rec.Header().Get(kedahttp.HeaderColdStart), tt.wantColdStartHeader; got != want {
				t.Fatalf("cold-start header = %q, want %q", got, want)
			}
		})
	}
}

func TestEndpointResolver_ReadinessTimeout(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready - simulates a backend with no replicas.

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 25 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, defaultIR())
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called on timeout")
	}
	if got, want := rec.Code, http.StatusGatewayTimeout; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_Fallback(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready - simulates a backend with no replicas.

	fallbackURL := &url.URL{Host: "fallback"}

	var nextCalled bool
	var capturedUpstream *url.URL
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		capturedUpstream = util.UpstreamURLFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		Fallback: &httpv1beta1.TargetRef{Service: "fallback"},
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 25 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)
	ctx := util.ContextWithFallbackURL(req.Context(), fallbackURL)
	req = req.WithContext(ctx)

	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called with fallback")
	}
	if *capturedUpstream != *fallbackURL {
		t.Fatalf("upstream = %v, want %v", capturedUpstream, fallbackURL)
	}
}

func TestEndpointResolver_FallbackConfiguredButUpstreamReady(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	addReadyEndpoint(cache)

	upstreamURL := &url.URL{Host: "upstream"}
	fallbackURL := &url.URL{Host: "fallback"}

	var capturedUpstream *url.URL
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUpstream = util.UpstreamURLFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		Fallback: &httpv1beta1.TargetRef{Service: "fallback"},
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 5 * time.Second,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)
	ctx := util.ContextWithFallbackURL(req.Context(), fallbackURL)
	req = req.WithContext(ctx)

	mw.ServeHTTP(rec, req)

	if *capturedUpstream != *upstreamURL {
		t.Fatalf("upstream = %v, want %v", capturedUpstream, upstreamURL)
	}
}

func TestEndpointResolver_ColdStart(t *testing.T) {
	tests := map[string]struct {
		enableColdStartHeader bool
		wantColdStartHeader   string
	}{
		"with cold-start header": {
			enableColdStartHeader: true,
			wantColdStartHeader:   "true",
		},
		"without cold-start header": {
			enableColdStartHeader: false,
			wantColdStartHeader:   "",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cache := k8s.NewReadyEndpointsCache(logr.Discard())
			// Start with no ready endpoints

			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
				ReadinessTimeout:      2 * time.Second,
				EnableColdStartHeader: tt.enableColdStartHeader,
			})

			rec := httptest.NewRecorder()
			req := newRequest(t, defaultIR())

			// Mark ready after WaitForReady starts blocking, so the middleware
			// observes a cold start. This mirrors the pattern used in
			// ReadyEndpointsCache's own tests.
			go func() {
				time.Sleep(100 * time.Millisecond)
				addReadyEndpoint(cache)
			}()

			mw.ServeHTTP(rec, req)

			if !nextCalled {
				t.Fatal("expected next handler to be called after cold start")
			}
			if got, want := rec.Header().Get(kedahttp.HeaderColdStart), tt.wantColdStartHeader; got != want {
				t.Fatalf("cold-start header = %q, want %q", got, want)
			}
		})
	}
}

func TestEndpointResolver_FallbackWithPerRouteReadinessOverride(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready — backend has no replicas.

	fallbackURL := &url.URL{Host: "fallback"}

	var nextCalled bool
	var capturedUpstream *url.URL
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		capturedUpstream = util.UpstreamURLFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		Fallback: &httpv1beta1.TargetRef{Service: "fallback"},
	}
	// Set a short readiness timeout to trigger fast fallback
	ir.Spec.Timeouts.Readiness = &metav1.Duration{Duration: 25 * time.Millisecond}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 0,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)

	ctx := util.ContextWithFallbackURL(req.Context(), fallbackURL)
	req = req.WithContext(ctx)

	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called with fallback")
	}
	if capturedUpstream == nil || *capturedUpstream != *fallbackURL {
		t.Fatalf("upstream = %v, want %v", capturedUpstream, fallbackURL)
	}
}

func TestEndpointResolver_FallbackDeadContextReturns504(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready — backend has no replicas.

	fallbackURL := &url.URL{Host: "fallback"}

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		Fallback: &httpv1beta1.TargetRef{Service: "fallback"},
	}

	// Set readiness timeout equal to the request timeout so the parent
	// context is dead by the time the fallback path runs.
	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 50 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)

	ctx := util.ContextWithFallbackURL(req.Context(), fallbackURL)
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called when context is dead")
	}
	if got, want := rec.Code, http.StatusGatewayTimeout; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_ZeroReadinessUsesParentDeadline(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready — backend has no replicas.

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 0, // no dedicated readiness deadline
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, defaultIR())

	// Parent context with a short deadline acts as the only bound.
	ctx, cancel := context.WithTimeout(req.Context(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called on parent context timeout")
	}
	if got, want := rec.Code, http.StatusGatewayTimeout; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_RouteSpecReadinessOverride(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready - simulates a backend with no replicas.

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	ir := defaultIR()
	ir.Spec.Timeouts.Readiness = &metav1.Duration{Duration: 25 * time.Millisecond}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 5 * time.Second, // global default — should be overridden
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called when route spec readiness times out")
	}
	if got, want := rec.Code, http.StatusGatewayTimeout; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_PerRouteZeroReadinessDisablesGlobal(t *testing.T) {
	// synctest provides a fake clock so the simulated 10s cold start completes instantly.
	synctest.Test(t, func(t *testing.T) {
		cache := k8s.NewReadyEndpointsCache(logr.Discard())

		var nextCalled bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		ir := defaultIR()
		ir.Spec.Timeouts.Readiness = &metav1.Duration{Duration: 0}

		mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
			ReadinessTimeout: 5 * time.Second,
		})

		rec := httptest.NewRecorder()
		req := newRequest(t, ir)
		ctx, cancel := context.WithTimeout(req.Context(), time.Minute)
		defer cancel()
		req = req.WithContext(ctx)

		// Add the ready endpoint after the global readiness timeout to prove that
		// that disabling it via IR.Timeouts.Readiness=0 works.
		go func() {
			time.Sleep(10 * time.Second)
			addReadyEndpoint(cache)
		}()

		mw.ServeHTTP(rec, req)

		if !nextCalled {
			t.Fatal("per-route zero readiness should disable the global timeout")
		}
		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
	})
}

func TestEndpointResolver_StreamingCallbackDuringColdStart(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"real\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n"))
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		StreamingCallback: &httpv1beta1.StreamingCallbackSpec{
			Message:  "Model is waking up...",
			Interval: metav1.Duration{Duration: 100 * time.Millisecond},
		},
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout:      5 * time.Second,
		EnableColdStartHeader: true,
	})

	rec := httptest.NewRecorder()
	req := newStreamingRequest(t, ir, `{"stream":true,"messages":[]}`)

	// Mark ready after a short delay to simulate cold start.
	go func() {
		time.Sleep(150 * time.Millisecond)
		addReadyEndpoint(cache)
	}()

	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called after cold start")
	}
	if got, want := rec.Header().Get("Content-Type"), "text/event-stream"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Model is waking up...") {
		t.Fatalf("response body should contain callback message, got:\n%s", body)
	}
	if !strings.Contains(body, `"id":"real"`) {
		t.Fatalf("response body should contain upstream response, got:\n%s", body)
	}
}

func TestEndpointResolver_StreamingCallbackNonStreamingRequest(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		StreamingCallback: &httpv1beta1.StreamingCallbackSpec{
			Message:  "Model is waking up...",
			Interval: metav1.Duration{Duration: 100 * time.Millisecond},
		},
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 25 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	// Non-streaming request — "stream": false
	req := newStreamingRequest(t, ir, `{"stream":false,"messages":[]}`)
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called on timeout")
	}
	// Should use normal path and return timeout error
	if got, want := rec.Code, http.StatusGatewayTimeout; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_StreamingCallbackTimeout(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	ir := defaultIR()
	ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
		StreamingCallback: &httpv1beta1.StreamingCallbackSpec{
			Message:  "Loading...",
			Interval: metav1.Duration{Duration: 50 * time.Millisecond},
		},
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 100 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	req := newStreamingRequest(t, ir, `{"stream":true}`)
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called on timeout")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Loading...") {
		t.Fatalf("response body should contain callback message, got:\n%s", body)
	}
	if !strings.Contains(body, "Backend did not become ready") {
		t.Fatalf("response body should contain error message, got:\n%s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("response body should contain [DONE] marker, got:\n%s", body)
	}
}

func TestEndpointResolver_StreamingCallbackNotConfigured(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	addReadyEndpoint(cache)

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// No StreamingCallback configured
	ir := defaultIR()

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		ReadinessTimeout: 5 * time.Second,
	})

	rec := httptest.NewRecorder()
	req := newStreamingRequest(t, ir, `{"stream":true}`)
	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called (warm path)")
	}
	// Should NOT have SSE content type
	if got := rec.Header().Get("Content-Type"); got == "text/event-stream" {
		t.Fatal("should not use SSE when StreamingCallback is not configured")
	}
}

func TestIsStreamingRequest(t *testing.T) {
	tests := map[string]struct {
		body string
		want bool
	}{
		"stream true":     {body: `{"stream":true,"model":"test"}`, want: true},
		"stream false":    {body: `{"stream":false}`, want: false},
		"no stream field": {body: `{"model":"test"}`, want: false},
		"empty body":      {body: "", want: false},
		"invalid json":    {body: `not json`, want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			} else {
				req = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(tt.body))
			}
			got, err := isStreamingRequest(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("isStreamingRequest() = %v, want %v", got, tt.want)
			}
			// Verify body is restored for non-empty inputs
			if tt.body != "" && req.Body != nil {
				restored := new(bytes.Buffer)
				_, _ = restored.ReadFrom(req.Body)
				if restored.String() != tt.body {
					t.Fatalf("body not restored: got %q, want %q", restored.String(), tt.body)
				}
			}
		})
	}
}

func newStreamingRequest(t *testing.T, ir *httpv1beta1.InterceptorRoute, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	ctx = util.ContextWithUpstreamURL(ctx, &url.URL{Host: "upstream"})
	req = req.WithContext(ctx)
	return req
}

func newRequest(t *testing.T, ir *httpv1beta1.InterceptorRoute) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	ctx = util.ContextWithUpstreamURL(ctx, &url.URL{Host: "upstream"})
	req = req.WithContext(ctx)
	return req
}

func defaultIR() *httpv1beta1.InterceptorRoute {
	return &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{Service: testService},
		},
	}
}

func addReadyEndpoint(cache *k8s.ReadyEndpointsCache) {
	cache.Update(testNamespace+"/"+testService, []*discov1.EndpointSlice{
		{Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}}},
	})
}
