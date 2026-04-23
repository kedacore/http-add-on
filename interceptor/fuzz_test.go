package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

// FuzzParseTLSVersion exercises the TLS version parser with arbitrary strings.
// The function parses user-supplied configuration, so it must never panic.
func FuzzParseTLSVersion(f *testing.F) {
	f.Add("1.2")
	f.Add("1.3")
	f.Add("")
	f.Add("1.0")
	f.Add("1.1")
	f.Add("TLSv1.2")
	f.Add("ssl3")

	f.Fuzz(func(t *testing.T, v string) {
		_, _ = parseTLSVersion(v)
	})
}

// FuzzParseCipherSuites exercises cipher-suite name parsing with arbitrary
// strings. The function parses user-supplied configuration, so it must never
// panic.
func FuzzParseCipherSuites(f *testing.F) {
	f.Add("TLS_AES_128_GCM_SHA256")
	f.Add("TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384")
	f.Add("")
	f.Add(",,,")
	f.Add("UNKNOWN_SUITE")
	f.Add("TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseCipherSuites(s)
	})
}

// FuzzParseCurvePreferences exercises curve preference parsing with arbitrary
// strings. The function parses user-supplied configuration, so it must never
// panic.
func FuzzParseCurvePreferences(f *testing.F) {
	f.Add("CurveP256")
	f.Add("X25519")
	f.Add("P-256,P-384")
	f.Add("")
	f.Add(",,,")
	f.Add("UNKNOWN_CURVE")
	f.Add("CurveP256, X25519, CurveP384")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseCurvePreferences(s)
	})
}

// FuzzProxyHandler sends fuzzed HTTP requests through the full proxy handler
// chain. This exercises routing, queue interaction, and upstream proxying with
// arbitrary method/host/path/header combinations. The primary goal is crash
// detection (no panics, deadlocks, or goroutine leaks). Additionally, we
// verify that requests to the known host are proxied successfully and requests
// to unknown hosts receive an error response.
func FuzzProxyHandler(f *testing.F) {
	f.Add("GET", "fuzz.example.com", "/api/v1", "X-Custom", "value")
	f.Add("POST", "fuzz.example.com", "/", "", "")
	f.Add("DELETE", "unknown.host", "/path", "", "")
	f.Add("GET", "", "", "", "")
	f.Add("PATCH", "fuzz.example.com", "/api/../secret", "Host", "evil.com")
	f.Add("GET", "[::1]:8080", "/path", "X-Forwarded-For", "127.0.0.1")

	const fuzzHost = "fuzz.example.com"
	const fuzzServiceKey = "fuzz-ns/fuzz-service"

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	f.Cleanup(backend.Close)

	fakeQueue := queue.NewFakeCounterBuffered()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-fakeQueue.ResizedCh:
			case <-done:
				return
			}
		}
	}()
	f.Cleanup(func() { close(done) })

	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
	readyCache.Update(fuzzServiceKey, []*discov1.EndpointSlice{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fuzz-slice",
			Namespace: "fuzz-ns",
			Labels:    map[string]string{discov1.LabelServiceName: "fuzz-service"},
		},
		Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
	}})

	routingTable := routingtest.NewTable()
	routingTable.Memory[fuzzHost] = &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "fuzz-ir", Namespace: "fuzz-ns"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{Service: "fuzz-service", Port: 80},
		},
	}

	handler := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       logr.Discard(),
		Queue:        fakeQueue,
		ReadyCache:   readyCache,
		RoutingTable: routingTable,
		Reader:       fake.NewClientBuilder().WithScheme(kedacache.NewScheme()).Build(),
		Timeouts: config.Timeouts{
			Connect:        500 * time.Millisecond,
			Readiness:      1 * time.Second,
			Request:        5 * time.Second,
			ResponseHeader: 2 * time.Second,
		},
		Serving:             config.Serving{},
		Tracing:             config.Tracing{},
		Instruments:         metrics.NewNoopInstruments(),
		dialAddressOverride: backend.Listener.Addr().String(),
	})

	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT"}

	f.Fuzz(func(t *testing.T, method, host, path, headerKey, headerVal string) {
		if len(method) == 0 {
			method = "GET"
		} else {
			method = validMethods[int(method[0])%len(validMethods)]
		}

		if path == "" || path[0] != '/' {
			path = "/" + path
		}

		req, err := http.NewRequest(method, path, nil)
		if err != nil {
			return
		}
		req.Host = host
		req.RequestURI = path
		if headerKey != "" {
			req.Header.Set(headerKey, headerVal)
		}

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		code := rec.Code

		if host == fuzzHost {
			// The route must be found (not 404). Upstream errors like 502/504
			// are expected when fuzzed headers break the proxy transport.
			if code == http.StatusNotFound {
				t.Errorf("request to known host %q returned 404, route should have matched", host)
			}
		} else if host != "" {
			// Unknown hosts must hit the routing layer's 404.
			if code != http.StatusNotFound {
				t.Errorf("request to unknown host %q returned %d, want 404", host, code)
			}
		}
	})
}
