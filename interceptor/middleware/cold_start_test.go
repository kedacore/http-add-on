package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

func TestColdStart_BackendReady(t *testing.T) {
	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
	addReadyEndpoint(readyCache)

	body := loadingBody
	ir := placeholderIR(&httpv1beta1.StaticResponse{
		StatusCode: http.StatusServiceUnavailable,
		Body:       &body,
	})

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := newColdStartMiddleware(next, readyCache, nil)

	rec := httptest.NewRecorder()
	req := newColdStartRequest(t, ir, "/")
	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called when backend is ready")
	}
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestColdStart_BackendNotReady(t *testing.T) {
	t.Run("NoPlaceholder", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Target: httpv1beta1.TargetRef{Service: testService},
			},
		}

		var nextCalled bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		mw := newColdStartMiddleware(next, readyCache, nil)

		rec := httptest.NewRecorder()
		req := newColdStartRequest(t, ir, "/")
		mw.ServeHTTP(rec, req)

		if !nextCalled {
			t.Fatal("expected next handler to be called when no placeholder configured")
		}
	})

	t.Run("WithResponse", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		body := `{"error": "loading"}`
		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusServiceUnavailable,
			Body:       &body,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		})

		var nextCalled bool
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		mw := newColdStartMiddleware(next, readyCache, nil)

		rec := httptest.NewRecorder()
		req := newColdStartRequest(t, ir, "/")
		mw.ServeHTTP(rec, req)

		if nextCalled {
			t.Fatal("expected next handler NOT to be called when placeholder response is served")
		}
		if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
		if got, want := rec.Body.String(), body; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
		if got, want := rec.Header().Get("Content-Type"), "application/json"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}
	})

	t.Run("EmptyBody", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusOK,
		})

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		mw := newColdStartMiddleware(next, readyCache, nil)

		rec := httptest.NewRecorder()
		req := newColdStartRequest(t, ir, "/")
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
		if got := rec.Body.String(); got != "" {
			t.Fatalf("body = %q, want empty", got)
		}
	})

	t.Run("ConfigMapNotFound", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusServiceUnavailable,
			BodyFromConfigMap: &httpv1beta1.ConfigMapKeyRef{
				Name: "missing-cm",
				Key:  "page.html",
			},
		})

		reader := fake.NewClientBuilder().WithScheme(cache.NewScheme()).Build()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		mw := newColdStartMiddleware(next, readyCache, reader)

		rec := httptest.NewRecorder()
		req := newColdStartRequest(t, ir, "/")
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
	})
}

// TestColdStart_PendingLimitOptIn verifies that holding requests instead of
// serving the placeholder immediately is opt-in: it requires both an explicit
// per-route limit and overflow: Placeholder.
func TestColdStart_PendingLimitOptIn(t *testing.T) {
	body := loadingBody
	limit := int32(100)

	tests := map[string]struct {
		coldStart      *httpv1beta1.ColdStartSpec
		wantNextCalled bool
	}{
		"PlaceholderOnlyServesImmediately": {
			coldStart: &httpv1beta1.ColdStartSpec{
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: &httpv1beta1.StaticResponse{Body: &body},
				},
			},
			wantNextCalled: false,
		},
		"LimitWithoutPlaceholderOverflowServesImmediately": {
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxPendingRequests: &limit,
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: &httpv1beta1.StaticResponse{Body: &body},
				},
			},
			wantNextCalled: false,
		},
		"PlaceholderOverflowWithLimitHoldsRequests": {
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxPendingRequests: &limit,
				Overflow:           httpv1beta1.ColdStartOverflowPlaceholder,
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: &httpv1beta1.StaticResponse{Body: &body},
				},
			},
			wantNextCalled: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target:    httpv1beta1.TargetRef{Service: testService},
					ColdStart: tc.coldStart,
				},
			}

			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			mw := newColdStartMiddleware(next, readyCache, nil)

			rec := httptest.NewRecorder()
			req := newColdStartRequest(t, ir, "/")
			mw.ServeHTTP(rec, req)

			if got := nextCalled; got != tc.wantNextCalled {
				t.Fatalf("next called: got %v, want %v", got, tc.wantNextCalled)
			}
			if !tc.wantNextCalled {
				if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
					t.Fatalf("status code = %d, want %d", got, want)
				}
				if got := rec.Body.String(); got != body {
					t.Fatalf("body = %q, want %q", got, body)
				}
			}
		})
	}
}

// TestColdStart_PendingLimit exercises the admission limit end-to-end through
// the production chain order Counting -> ColdStart, so the counter includes
// the request being evaluated.
func TestColdStart_PendingLimit(t *testing.T) {
	ptr := func(i int32) *int32 { return &i }
	body := loadingBody

	tests := map[string]struct {
		globalLimit    int
		coldStart      *httpv1beta1.ColdStartSpec
		warm           bool
		prefill        int
		wantStatus     int
		wantNextCalled bool
		wantBody       string
	}{
		"UnlimitedByDefault": {
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"AdmitsBelowLimit": {
			globalLimit:    2,
			prefill:        1,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"RejectsAtLimit": {
			globalLimit: 2,
			prefill:     2,
			wantStatus:  http.StatusServiceUnavailable,
			wantBody:    "too many pending requests\n",
		},
		"WarmBackendBypassesLimit": {
			globalLimit:    1,
			warm:           true,
			prefill:        5,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"PerRouteLimitOverridesGlobalUnlimited": {
			coldStart:  &httpv1beta1.ColdStartSpec{MaxPendingRequests: ptr(1)},
			prefill:    1,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "too many pending requests\n",
		},
		"PerRouteLimitOverridesGlobalLimit": {
			globalLimit:    1,
			coldStart:      &httpv1beta1.ColdStartSpec{MaxPendingRequests: ptr(5)},
			prefill:        2,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"PlaceholderOverflow": {
			globalLimit: 1,
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxPendingRequests: ptr(1),
				Overflow:           httpv1beta1.ColdStartOverflowPlaceholder,
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: &httpv1beta1.StaticResponse{Body: &body},
				},
			},
			prefill:    1,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   body,
		},
		"PlaceholderOverflowWithoutResponseFallsBackToReject": {
			globalLimit: 1,
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxPendingRequests: ptr(1),
				Overflow:           httpv1beta1.ColdStartOverflowPlaceholder,
			},
			prefill:    1,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "too many pending requests\n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "test-route"},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target:    httpv1beta1.TargetRef{Service: testService},
					ColdStart: tc.coldStart,
				},
			}

			counter := queue.NewFakeCounterBuffered()
			readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
			if tc.warm {
				addReadyEndpoint(readyCache)
			}

			key := k8s.ResourceKey(ir.Namespace, ir.Name)
			counter.EnsureKey(key)
			for i := 0; i < tc.prefill; i++ {
				if err := counter.Increase(key, 1); err != nil {
					t.Fatalf("Increase() error: %v", err)
				}
			}

			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			var h http.Handler = NewColdStart(next, readyCache, nil, counter, metrics.NewNoopInstruments(), ColdStartConfig{
				MaxPendingRequests: tc.globalLimit,
			})
			h = NewCounting(h, counter, metrics.NewNoopInstruments())

			req := httptest.NewRequest("GET", "/test", nil)
			ctx := util.ContextWithLogger(req.Context(), logr.Discard())
			ctx = util.ContextWithInterceptorRoute(ctx, ir)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got, want := rec.Code, tc.wantStatus; got != want {
				t.Fatalf("status: got %d, want %d", got, want)
			}
			if got := nextCalled; got != tc.wantNextCalled {
				t.Fatalf("next called: got %v, want %v", got, tc.wantNextCalled)
			}
			if tc.wantBody != "" {
				if got := rec.Body.String(); got != tc.wantBody {
					t.Fatalf("body: got %q, want %q", got, tc.wantBody)
				}
			}

			// Admitted or rejected, the concurrency returns to the prefill
			// level once the request completes.
			if got, want := counter.Concurrency(key), tc.prefill; got != want {
				t.Fatalf("concurrency after request: got %d, want %d", got, want)
			}
		})
	}
}

func TestColdStart_RecordsRejectionMetric(t *testing.T) {
	instruments, reader := testInstruments(t)

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "test-route"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{Service: testService},
		},
	}

	counter := queue.NewFakeCounterBuffered()
	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

	key := k8s.ResourceKey(ir.Namespace, ir.Name)
	counter.EnsureKey(key)
	if err := counter.Increase(key, 1); err != nil {
		t.Fatalf("Increase() error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler should not be called")
	})

	var h http.Handler = NewColdStart(next, readyCache, nil, counter, instruments, ColdStartConfig{
		MaxPendingRequests: 1,
	})
	h = NewCounting(h, counter, metrics.NewNoopInstruments())

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status: got %d, want %d", got, want)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	m := requireMetric(t, rm, metrics.MetricColdStartRejections)
	sum := m.Data.(metricdata.Sum[int64])
	if got := len(sum.DataPoints); got != 1 {
		t.Fatalf("expected 1 data point, got %d", got)
	}
	dp := sum.DataPoints[0]
	if got := dp.Value; got != 1 {
		t.Fatalf("rejection count: got %d, want 1", got)
	}
	assertStringAttr(t, dp.Attributes, metrics.AttrRouteName, "test-route")
	assertStringAttr(t, dp.Attributes, metrics.AttrRouteNamespace, testNamespace)
}

func TestColdStart_ConfigMapLookup(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "pages"},
		Data: map[string]string{
			"index.html": "<h1>Loading</h1>",
			"styles.css": "body { color: red }",
		},
	}
	reader := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(cm).Build()

	tests := map[string]struct {
		key             string
		path            string
		headers         map[string]string
		wantStatus      int
		wantBody        string
		wantContentType string
	}{
		"PathDerivedRoot": {
			path:            "/",
			wantBody:        "<h1>Loading</h1>",
			wantContentType: "text/html; charset=utf-8",
			wantStatus:      http.StatusTeapot,
		},
		"PathDerivedFile": {
			path:            "/styles.css",
			wantBody:        "body { color: red }",
			wantContentType: "text/css; charset=utf-8",
		},
		"PathDerivedKeyNotFound": {
			path: "/missing.js",
		},
		"ExplicitKey": {
			key:             "styles.css",
			path:            "/ignored",
			wantBody:        "body { color: red }",
			wantContentType: "text/css; charset=utf-8",
		},
		"ExplicitKeyNotFound": {
			key:             "missing-key",
			path:            "/ignored",
			wantStatus:      http.StatusInternalServerError,
			wantBody:        "Internal Server Error\n",
			wantContentType: "text/plain; charset=utf-8",
		},
		"ExplicitContentTypeNotOverridden": {
			path:            "/",
			headers:         map[string]string{"Content-Type": "text/plain"},
			wantBody:        "<h1>Loading</h1>",
			wantContentType: "text/plain",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

			ir := placeholderIR(&httpv1beta1.StaticResponse{
				StatusCode: http.StatusTeapot,
				BodyFromConfigMap: &httpv1beta1.ConfigMapKeyRef{
					Name: "pages",
					Key:  tc.key,
				},
				Headers: tc.headers,
			})

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next handler should not be called")
			})
			mw := newColdStartMiddleware(next, readyCache, reader)

			rec := httptest.NewRecorder()
			req := newColdStartRequest(t, ir, tc.path)
			mw.ServeHTTP(rec, req)

			if tc.wantStatus != 0 {
				if got := rec.Code; got != tc.wantStatus {
					t.Fatalf("status code = %d, want %d", got, tc.wantStatus)
				}
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Fatalf("body = %q, want %q", got, tc.wantBody)
			}
			if got := rec.Header().Get("Content-Type"); got != tc.wantContentType {
				t.Fatalf("Content-Type = %q, want %q", got, tc.wantContentType)
			}
		})
	}
}

func TestConfigMapKeyFromPath(t *testing.T) {
	tests := map[string]struct {
		path string
		want string
	}{
		"Root":       {path: "/", want: "index.html"},
		"SingleFile": {path: "/styles.css", want: "styles.css"},
		"NestedPath": {path: "/assets/styles.css", want: "assets/styles.css"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := configMapKeyFromPath(tc.path)
			if got != tc.want {
				t.Fatalf("configMapKeyFromPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func placeholderIR(resp *httpv1beta1.StaticResponse) *httpv1beta1.InterceptorRoute {
	return &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{Service: testService},
			ColdStart: &httpv1beta1.ColdStartSpec{
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: resp,
				},
			},
		},
	}
}

func newColdStartMiddleware(next http.Handler, readyCache *k8s.ReadyEndpointsCache, reader client.Reader) *ColdStart {
	return NewColdStart(next, readyCache, reader, queue.NewMemory(), metrics.NewNoopInstruments(), ColdStartConfig{})
}

func newColdStartRequest(t *testing.T, ir *httpv1beta1.InterceptorRoute, urlPath string) *http.Request {
	t.Helper()

	req := httptest.NewRequest("GET", urlPath, nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	return req.WithContext(ctx)
}
