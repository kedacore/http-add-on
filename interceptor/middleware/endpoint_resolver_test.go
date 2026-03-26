package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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
				WaitTimeout:           5 * time.Second,
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

func TestEndpointResolver_WaitTimeout(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		WaitTimeout: 25 * time.Millisecond,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, defaultIR())
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called on timeout")
	}
	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestEndpointResolver_Fallback(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready

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
		WaitTimeout: 25 * time.Millisecond,
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
		WaitTimeout: 5 * time.Second,
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
				WaitTimeout:           2 * time.Second,
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

func TestEndpointResolver_WaitTimeoutAnnotationOverride(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	// Do not mark ready — the test verifies that the annotation-overridden
	// wait timeout (very short) causes a 502 instead of using the long default.

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	ir := defaultIR()
	ir.Annotations = map[string]string{
		k8s.AnnotationConditionWaitTimeout: "25ms",
	}

	mw := NewEndpointResolver(next, cache, EndpointResolverConfig{
		WaitTimeout: 5 * time.Second,
	})

	rec := httptest.NewRecorder()
	req := newRequest(t, ir)
	mw.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("expected next handler not to be called when annotation overrides wait timeout")
	}
	if got, want := rec.Code, http.StatusBadGateway; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
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
