package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

func TestPlaceholder_BackendReady(t *testing.T) {
	cache := k8s.NewReadyEndpointsCache(logr.Discard())
	addReadyEndpoint(cache)

	body := `{"loading": true}`
	ir := placeholderIR(&httpv1beta1.StaticResponse{
		StatusCode: http.StatusServiceUnavailable,
		Body:       &body,
	})

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewPlaceholder(next, cache, nil)

	rec := httptest.NewRecorder()
	req := newPlaceholderRequest(t, ir)
	mw.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called when backend is ready")
	}
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestPlaceholder_BackendNotReady(t *testing.T) {
	t.Run("NoPlaceholder", func(t *testing.T) {
		cache := k8s.NewReadyEndpointsCache(logr.Discard())

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

		mw := NewPlaceholder(next, cache, nil)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
		mw.ServeHTTP(rec, req)

		if !nextCalled {
			t.Fatal("expected next handler to be called when no placeholder configured")
		}
	})

	t.Run("WithResponse", func(t *testing.T) {
		cache := k8s.NewReadyEndpointsCache(logr.Discard())

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

		mw := NewPlaceholder(next, cache, nil)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
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
		cache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusOK,
		})

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		mw := NewPlaceholder(next, cache, nil)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusOK; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
		if got := rec.Body.String(); got != "" {
			t.Fatalf("body = %q, want empty", got)
		}
	})

	t.Run("BodyFromConfigMap", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusServiceUnavailable,
			BodyFromConfigMap: &httpv1beta1.ConfigMapKeyRef{
				Name: "my-cm",
				Key:  "loading.html",
			},
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
		})

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "my-cm"},
			Data:       map[string]string{"loading.html": "<h1>Loading...</h1>"},
		}
		reader := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(cm).Build()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		mw := NewPlaceholder(next, readyCache, reader)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusServiceUnavailable; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
		if got, want := rec.Body.String(), "<h1>Loading...</h1>"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
		if got, want := rec.Header().Get("Content-Type"), "text/html"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
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

		mw := NewPlaceholder(next, readyCache, reader)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
	})

	t.Run("ConfigMapKeyNotFound", func(t *testing.T) {
		readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

		ir := placeholderIR(&httpv1beta1.StaticResponse{
			StatusCode: http.StatusServiceUnavailable,
			BodyFromConfigMap: &httpv1beta1.ConfigMapKeyRef{
				Name: "my-cm",
				Key:  "missing-key",
			},
		})

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "my-cm"},
			Data:       map[string]string{"other-key": "other-value"},
		}
		reader := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(cm).Build()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		mw := NewPlaceholder(next, readyCache, reader)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequest(t, ir)
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
	})
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

func newPlaceholderRequest(t *testing.T, ir *httpv1beta1.InterceptorRoute) *http.Request {
	t.Helper()

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	return req.WithContext(ctx)
}
