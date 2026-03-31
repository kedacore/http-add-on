package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
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

	var concurrencyDuringRequest int
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		concurrencyDuringRequest = currentConcurrency(t, counter)
		w.WriteHeader(http.StatusOK)
	})

	mw := NewCounting(next, counter, metrics.NewNoopInstruments())

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

func currentConcurrency(t *testing.T, counter *queue.FakeCounter) int {
	t.Helper()

	counts, err := counter.Current()
	if err != nil {
		t.Fatalf("counter.Current() error: %v", err)
	}
	if got, want := len(counts.Counts), 1; got != want {
		t.Fatalf("expected %d counter entry, got %d", want, got)
	}
	for _, c := range counts.Counts {
		return c.Concurrency
	}

	return 0
}
