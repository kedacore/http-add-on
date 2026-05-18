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
	req := newPlaceholderRequestWithPath(t, ir, "/")
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
		req := newPlaceholderRequestWithPath(t, ir, "/")
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
		req := newPlaceholderRequestWithPath(t, ir, "/")
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
		req := newPlaceholderRequestWithPath(t, ir, "/")
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

		mw := NewPlaceholder(next, readyCache, reader)

		rec := httptest.NewRecorder()
		req := newPlaceholderRequestWithPath(t, ir, "/")
		mw.ServeHTTP(rec, req)

		if got, want := rec.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status code = %d, want %d", got, want)
		}
	})
}

func TestPlaceholder_ConfigMapLookup(t *testing.T) {
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
			mw := NewPlaceholder(next, readyCache, reader)

			rec := httptest.NewRecorder()
			req := newPlaceholderRequestWithPath(t, ir, tc.path)
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

func newPlaceholderRequestWithPath(t *testing.T, ir *httpv1beta1.InterceptorRoute, urlPath string) *http.Request {
	t.Helper()

	req := httptest.NewRequest("GET", urlPath, nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	return req.WithContext(ctx)
}
