package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

func TestStaticRouting(t *testing.T) {
	healthzRoute := httpv1beta1.StaticRoute{
		Rules: []httpv1beta1.RoutingRule{
			{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
		},
		Response: httpv1beta1.StaticResponse{StatusCode: 200},
	}

	tests := map[string]struct {
		routes       []httpv1beta1.StaticRoute
		backendReady bool
		path         string
		wantNext     bool
		wantUpstream bool
	}{
		"no static routes": {
			path:     "/anything",
			wantNext: true,
		},
		"no match": {
			routes:   []httpv1beta1.StaticRoute{healthzRoute},
			path:     "/other",
			wantNext: true,
		},
		"match with backend ready": {
			routes:       []httpv1beta1.StaticRoute{healthzRoute},
			backendReady: true,
			path:         "/healthz",
			wantUpstream: true,
		},
		"match with backend not ready": {
			routes: []httpv1beta1.StaticRoute{healthzRoute},
			path:   "/healthz",
		},
		"response mode always skips backend": {
			routes: []httpv1beta1.StaticRoute{{
				Rules: []httpv1beta1.RoutingRule{
					{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
				},
				Response:     httpv1beta1.StaticResponse{StatusCode: 200},
				ResponseMode: httpv1beta1.StaticRouteResponseModeAlways,
			}},
			backendReady: true,
			path:         "/healthz",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
			if tc.backendReady {
				addReadyEndpoint(readyCache)
			}

			var ir *httpv1beta1.InterceptorRoute
			if tc.routes != nil {
				ir = staticRouteIR(tc.routes...)
			} else {
				ir = defaultIR()
			}

			var nextCalled, upstreamCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})
			upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalled = true
				w.WriteHeader(http.StatusOK)
			})

			mw := NewStaticRouting(next, upstream, readyCache, nil)
			rec := httptest.NewRecorder()
			req := newStaticRouteRequest(t, ir, http.MethodGet, tc.path)
			mw.ServeHTTP(rec, req)

			if nextCalled != tc.wantNext {
				t.Fatalf("next called = %v, want %v", nextCalled, tc.wantNext)
			}
			if upstreamCalled != tc.wantUpstream {
				t.Fatalf("upstream called = %v, want %v", upstreamCalled, tc.wantUpstream)
			}
		})
	}
}

func TestStaticRouting_StaticResponse(t *testing.T) {
	healthzRules := []httpv1beta1.RoutingRule{
		{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
	}

	tests := map[string]struct {
		routes      []httpv1beta1.StaticRoute
		configMap   *corev1.ConfigMap
		path        string
		wantCode    int
		wantBody    string
		wantHeaders map[string]string
	}{
		"serves configured response": {
			routes: []httpv1beta1.StaticRoute{{
				Rules: healthzRules,
				Response: httpv1beta1.StaticResponse{
					StatusCode: http.StatusOK,
					Body:       ptr.To("service unavailable"),
					Headers:    map[string]string{"Custom-Header": "test"},
				},
			}},
			path:        "/healthz",
			wantCode:    http.StatusOK,
			wantBody:    "service unavailable",
			wantHeaders: map[string]string{"Custom-Header": "test"},
		},
		"default status code is 200": {
			routes: []httpv1beta1.StaticRoute{{
				Rules:    healthzRules,
				Response: httpv1beta1.StaticResponse{},
			}},
			path:     "/healthz",
			wantCode: http.StatusOK,
		},
		"body from configmap": {
			routes: []httpv1beta1.StaticRoute{{
				Rules: healthzRules,
				Response: httpv1beta1.StaticResponse{
					StatusCode:        http.StatusServiceUnavailable,
					BodyFromConfigMap: &httpv1beta1.ConfigMapKeyRef{Name: "pages", Key: "status.html"},
				},
			}},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "pages"},
				Data:       map[string]string{"status.html": "<h1>Down</h1>"},
			},
			path:     "/healthz",
			wantBody: "<h1>Down</h1>",
		},
		"first match wins": {
			routes: []httpv1beta1.StaticRoute{
				{
					Rules: healthzRules,
					Response: httpv1beta1.StaticResponse{
						StatusCode: http.StatusOK,
						Body:       ptr.To("first"),
					},
				},
				{
					Rules: healthzRules,
					Response: httpv1beta1.StaticResponse{
						StatusCode: http.StatusOK,
						Body:       ptr.To("second"),
					},
				},
			},
			path:     "/healthz",
			wantBody: "first",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
			ir := staticRouteIR(tc.routes...)

			var reader client.Reader
			if tc.configMap != nil {
				reader = fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(tc.configMap).Build()
			}

			mw := NewStaticRouting(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("next called") }),
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("upstream called") }),
				readyCache, reader,
			)
			rec := httptest.NewRecorder()
			req := newStaticRouteRequest(t, ir, http.MethodGet, tc.path)
			mw.ServeHTTP(rec, req)

			if tc.wantCode != 0 {
				if got := rec.Code; got != tc.wantCode {
					t.Fatalf("status code = %d, want %d", got, tc.wantCode)
				}
			}
			if tc.wantBody != "" {
				if got := rec.Body.String(); got != tc.wantBody {
					t.Fatalf("body = %q, want %q", got, tc.wantBody)
				}
			}
			for k, want := range tc.wantHeaders {
				if got := rec.Header().Get(k); got != want {
					t.Fatalf("header %s = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestMatchStaticRoute(t *testing.T) {
	tests := map[string]struct {
		routes []httpv1beta1.StaticRoute
		path   string
		want   bool
	}{
		"match": {
			routes: []httpv1beta1.StaticRoute{
				{Rules: []httpv1beta1.RoutingRule{{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}}}},
			},
			path: "/healthz",
			want: true,
		},
		"no match": {
			routes: []httpv1beta1.StaticRoute{
				{Rules: []httpv1beta1.RoutingRule{{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}}}},
			},
			path: "/other",
			want: false,
		},
		"multiple rules OR semantics": {
			routes: []httpv1beta1.StaticRoute{
				{Rules: []httpv1beta1.RoutingRule{
					{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
					{Paths: []httpv1beta1.PathMatch{{Value: "/readyz"}}},
				}},
			},
			path: "/readyz",
			want: true,
		},
		"empty rule matches everything": {
			routes: []httpv1beta1.StaticRoute{
				{Rules: []httpv1beta1.RoutingRule{{}}},
			},
			path: "/anything",
			want: true,
		},
		"nil routes": {
			routes: nil,
			path:   "/anything",
			want:   false,
		},
		"second route matches": {
			routes: []httpv1beta1.StaticRoute{
				{Rules: []httpv1beta1.RoutingRule{{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}}}},
				{Rules: []httpv1beta1.RoutingRule{{Paths: []httpv1beta1.PathMatch{{Value: "/readyz"}}}}},
			},
			path: "/readyz",
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			got := matchStaticRoute(req, tc.routes)
			if (got != nil) != tc.want {
				t.Fatalf("matchStaticRoute() matched = %v, want %v", got != nil, tc.want)
			}
		})
	}
}

func staticRouteIR(routes ...httpv1beta1.StaticRoute) *httpv1beta1.InterceptorRoute {
	return &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target:       httpv1beta1.TargetRef{Service: testService},
			StaticRoutes: routes,
		},
	}
}

func newStaticRouteRequest(t *testing.T, ir *httpv1beta1.InterceptorRoute, method, urlPath string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, urlPath, nil)
	ctx := util.ContextWithLogger(req.Context(), logr.Discard())
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	return req.WithContext(ctx)
}
