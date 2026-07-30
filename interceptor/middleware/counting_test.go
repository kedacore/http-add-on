package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

func TestCounting_ConcurrencyTracking(t *testing.T) {
	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-route",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "test-svc",
				Port:    8080,
			},
		},
	}
	counter := queue.NewFakeCounterBuffered()
	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

	var concurrencyDuringRequest int
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		concurrencyDuringRequest = currentConcurrency(t, counter)
		w.WriteHeader(http.StatusOK)
	})

	mw := NewCounting(next, counter, metrics.NewNoopInstruments(), readyCache, nil, CountingConfig{})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status: got %d, want %d", got, want)
	}
	if got, want := concurrencyDuringRequest, 1; got != want {
		t.Fatalf("concurrency during request: got %d, want %d", got, want)
	}
	if got, want := currentConcurrency(t, counter), 0; got != want {
		t.Fatalf("concurrency after request: got %d, want %d", got, want)
	}
}

func TestCounting_HoldQueueCap(t *testing.T) {
	ptr := func(i int32) *int32 { return &i }
	body := loadingBody

	tests := map[string]struct {
		globalMaxDepth int
		coldStart      *httpv1beta1.ColdStartSpec
		warm           bool
		prefill        int
		wantStatus     int
		wantNextCalled bool
		wantRetryAfter bool
		wantBody       string
	}{
		"UnlimitedByDefault": {
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"AdmitsBelowCap": {
			globalMaxDepth: 2,
			prefill:        1,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"RejectsAtCap": {
			globalMaxDepth: 2,
			prefill:        2,
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: true,
			wantBody:       "cold-start hold queue full\n",
		},
		"WarmBackendBypassesCap": {
			globalMaxDepth: 1,
			warm:           true,
			prefill:        5,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"PerRouteCapOverridesGlobalUnlimited": {
			coldStart:      &httpv1beta1.ColdStartSpec{MaxQueueDepth: ptr(1)},
			prefill:        1,
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: true,
		},
		"PerRouteCapOverridesGlobalCap": {
			globalMaxDepth: 1,
			coldStart:      &httpv1beta1.ColdStartSpec{MaxQueueDepth: ptr(5)},
			prefill:        2,
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		"PlaceholderOverflow": {
			globalMaxDepth: 1,
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxQueueDepth: ptr(1),
				Overflow:      httpv1beta1.ColdStartOverflowPlaceholder,
				Placeholder: &httpv1beta1.ColdStartPlaceholder{
					Response: &httpv1beta1.StaticResponse{Body: &body},
				},
			},
			prefill:    1,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   body,
		},
		"PlaceholderOverflowWithoutResponseFallsBackToReject": {
			globalMaxDepth: 1,
			coldStart: &httpv1beta1.ColdStartSpec{
				MaxQueueDepth: ptr(1),
				Overflow:      httpv1beta1.ColdStartOverflowPlaceholder,
			},
			prefill:        1,
			wantStatus:     http.StatusServiceUnavailable,
			wantRetryAfter: true,
			wantBody:       "cold-start hold queue full\n",
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

			mw := NewCounting(next, counter, metrics.NewNoopInstruments(), readyCache, nil, CountingConfig{
				MaxQueueDepth: tc.globalMaxDepth,
			})

			req := httptest.NewRequest("GET", "/test", nil)
			ctx := util.ContextWithLogger(req.Context(), logr.Discard())
			ctx = util.ContextWithInterceptorRoute(ctx, ir)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if got, want := rec.Code, tc.wantStatus; got != want {
				t.Fatalf("status: got %d, want %d", got, want)
			}
			if got := nextCalled; got != tc.wantNextCalled {
				t.Fatalf("next called: got %v, want %v", got, tc.wantNextCalled)
			}
			if got := rec.Header().Get("Retry-After") != ""; got != tc.wantRetryAfter {
				t.Fatalf("Retry-After present: got %v, want %v", got, tc.wantRetryAfter)
			}
			if tc.wantBody != "" {
				if got := rec.Body.String(); got != tc.wantBody {
					t.Fatalf("body: got %q, want %q", got, tc.wantBody)
				}
			}

			// Rejected requests must not be counted: the concurrency stays at
			// the prefill level; admitted requests return to it after serving.
			if got, want := counter.Concurrency(key), tc.prefill; got != want {
				t.Fatalf("concurrency after request: got %d, want %d", got, want)
			}
		})
	}
}

func TestCounting_RecordsRejectionMetric(t *testing.T) {
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

	mw := NewCounting(next, counter, instruments, readyCache, nil, CountingConfig{MaxQueueDepth: 1})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status: got %d, want %d", got, want)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	m := requireMetric(t, rm, metrics.MetricColdStartRejected)
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

func currentConcurrency(t *testing.T, counter *queue.FakeCounter) int {
	t.Helper()

	counts, err := counter.Current()
	if err != nil {
		t.Fatalf("counter.Current() error: %v", err)
	}
	if got, want := len(counts), 1; got != want {
		t.Fatalf("expected %d counter entry, got %d", want, got)
	}
	for _, c := range counts {
		return c.Concurrency
	}

	return 0
}
