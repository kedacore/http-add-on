package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

const (
	testHost       = "test.example.com"
	testIRKey      = "test-namespace/test-httpso"
	testServiceKey = "test-namespace/test-service"
)

var testEndpointSlice = &discov1.EndpointSlice{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-service-slice",
		Namespace: "test-namespace",
		Labels:    map[string]string{discov1.LabelServiceName: "test-service"},
	},
	Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
}

func TestProxyHandler_SuccessfulRequest(t *testing.T) {
	h := newProxyTestHarness(t, harnessConfig{})

	resp := h.doRequest(t, http.MethodGet, "/api/resource", testHost)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestProxyHandler_ColdStartHeader(t *testing.T) {
	tests := map[string]struct {
		cfg        harnessConfig
		wantHeader string
	}{
		"enabled without cold start": {
			cfg:        harnessConfig{enableColdStartHeader: true},
			wantHeader: "false",
		},
		"enabled with cold start": {
			cfg:        harnessConfig{enableColdStartHeader: true, simulateColdStart: true},
			wantHeader: "true",
		},
		"disabled": {
			cfg:        harnessConfig{},
			wantHeader: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := newProxyTestHarness(t, tt.cfg)

			resp := h.doRequest(t, http.MethodGet, "/", testHost)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get(kedahttp.HeaderColdStart); got != tt.wantHeader {
				t.Errorf("cold-start header = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestProxyHandler_Tracing(t *testing.T) {
	tests := map[string]struct {
		tracingEnabled bool
	}{
		"enabled": {
			tracingEnabled: true,
		},
		"disabled": {
			tracingEnabled: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var receivedTraceparent string
			h := newProxyTestHarness(t, harnessConfig{
				tracingEnabled: tt.tracingEnabled,
				backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedTraceparent = r.Header.Get("Traceparent")
					w.WriteHeader(http.StatusOK)
				}),
			})

			resp := h.doRequest(t, http.MethodGet, "/", testHost)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			hasTraceparent := receivedTraceparent != ""
			if hasTraceparent != tt.tracingEnabled {
				t.Errorf("backend received Traceparent = %v, want %v", hasTraceparent, tt.tracingEnabled)
			}
		})
	}
}

func TestProxyHandler_TracingPreservesTraceID(t *testing.T) {
	const traceID = "12345678901234567890123456789012"
	incomingTraceparent := "00-" + traceID + "-1234567890123456-01"

	var receivedTraceparent string
	h := newProxyTestHarness(t, harnessConfig{
		tracingEnabled: true,
		backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedTraceparent = r.Header.Get("Traceparent")
			w.WriteHeader(http.StatusOK)
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = testHost
	req.Header.Set("Traceparent", incomingTraceparent)

	rec := httptest.NewRecorder()
	h.Handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(receivedTraceparent, traceID) {
		t.Errorf("expected trace ID %s to be preserved, got %q", traceID, receivedTraceparent)
	}
}

func TestProxyHandler_BackendReceivesCorrectRequest(t *testing.T) {
	var receivedMethod, receivedPath string
	h := newProxyTestHarness(t, harnessConfig{
		backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMethod = r.Method
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
		}),
	})

	resp := h.doRequest(t, http.MethodPost, "/api/data", testHost)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if receivedMethod != http.MethodPost {
		t.Errorf("backend received method = %q, want %q", receivedMethod, http.MethodPost)
	}
	if receivedPath != "/api/data" {
		t.Errorf("backend received path = %q, want %q", receivedPath, "/api/data")
	}
}

func TestProxyHandler_DisableKeepAlives(t *testing.T) {
	var backendRequestedClose bool
	h := newProxyTestHarness(t, harnessConfig{
		disableKeepAlives: true,
		backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backendRequestedClose = r.Close
			w.WriteHeader(http.StatusOK)
		}),
	})

	resp := h.doRequest(t, http.MethodGet, "/", testHost)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !backendRequestedClose {
		t.Error("expected backend request to set Connection: close when keep-alives are disabled")
	}
}

func TestProxyHandler_DefaultKeepAlivesEnabled(t *testing.T) {
	var backendRequestedClose bool
	h := newProxyTestHarness(t, harnessConfig{
		backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backendRequestedClose = r.Close
			w.WriteHeader(http.StatusOK)
		}),
	})

	resp := h.doRequest(t, http.MethodGet, "/", testHost)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if backendRequestedClose {
		t.Error("expected backend request to keep connections open when keep-alives are enabled")
	}
}

func TestProxyHandler_UnknownHostReturnsError(t *testing.T) {
	h := newProxyTestHarness(t, harnessConfig{})

	resp := h.doRequest(t, http.MethodGet, "/", "unknown.example.com")
	defer resp.Body.Close()

	// Unknown host should return 404 (routing middleware returns StatusNotFound)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestProxyHandler_QueueCounting(t *testing.T) {
	h := newProxyTestHarness(t, harnessConfig{useBlockingQueue: true})

	// Start request in background - DON'T close WaitCh yet
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp := h.doRequest(t, http.MethodGet, "/", testHost)
		_ = resp.Body.Close()
	}()

	// Wait for queue increment (blocking send from Increase)
	select {
	case event := <-h.Queue.ResizedCh:
		if event.Host != testIRKey {
			t.Errorf("expected host %s, got %s", testIRKey, event.Host)
		}
		if event.Count != 1 {
			t.Errorf("expected +1, got %d", event.Count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for queue increment")
	}

	// Unblock the readiness middleware by adding endpoints to the cache
	h.ReadyCache.Update(testServiceKey, []*discov1.EndpointSlice{testEndpointSlice})

	// Wait for queue decrement
	select {
	case event := <-h.Queue.ResizedCh:
		if event.Count != 1 {
			t.Errorf("expected decrement event count = 1, got %d", event.Count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for queue decrement")
	}

	// Wait for request to complete
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for request to complete")
	}

	// Verify final state
	counts, err := h.Queue.Current()
	if err != nil {
		t.Fatalf("failed to get queue counts: %v", err)
	}
	concurrency := counts.Counts[testIRKey].Concurrency
	if concurrency != 0 {
		t.Errorf("expected final concurrency to be 0, got %d", concurrency)
	}
}

func TestProxyHandler_TLSBackend(t *testing.T) {
	var receivedTLS bool
	h := newProxyTestHarness(t, harnessConfig{
		tlsEnabled: true,
		backendHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedTLS = r.TLS != nil
			w.WriteHeader(http.StatusOK)
		}),
	})

	resp := h.doRequest(t, http.MethodGet, "/api/resource", testHost)
	defer resp.Body.Close()

	if !receivedTLS {
		t.Error("expected backend to receive request over TLS")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// proxyTestHarness provides a configured proxy handler for testing.
type proxyTestHarness struct {
	Handler    http.Handler
	Backend    *httptest.Server
	Queue      *queue.FakeCounter
	ReadyCache *k8s.ReadyEndpointsCache
}

type harnessConfig struct {
	backendHandler        http.Handler
	enableColdStartHeader bool
	simulateColdStart     bool
	tlsEnabled            bool
	tracingEnabled        bool
	disableKeepAlives     bool
	useBlockingQueue      bool
}

// newProxyTestHarness creates a test harness with the full handler chain.
func newProxyTestHarness(t *testing.T, cfg harnessConfig) *proxyTestHarness {
	t.Helper()

	// Apply defaults
	if cfg.backendHandler == nil {
		cfg.backendHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	// Initialize OTel SDK for tracing tests
	if cfg.tracingEnabled {
		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second)))
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(tracing.NewPropagator())
	}

	// Create test backend
	var backend *httptest.Server
	if cfg.tlsEnabled {
		backend = httptest.NewTLSServer(cfg.backendHandler)
	} else {
		backend = httptest.NewServer(cfg.backendHandler)
	}
	t.Cleanup(backend.Close)

	// Create queue
	var fakeQueue *queue.FakeCounter
	if cfg.useBlockingQueue {
		fakeQueue = queue.NewFakeCounter()
	} else {
		fakeQueue = queue.NewFakeCounterBuffered()
	}

	// Create ready cache
	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

	// For non-blocking mode, pre-populate the cache so requests flow through immediately.
	// For cold start simulation, populate asynchronously so WaitForReady returns isColdStart=true.
	if !cfg.useBlockingQueue {
		if cfg.simulateColdStart {
			// Populate asynchronously: WaitForReady will block briefly, then return isColdStart=true
			go func() {
				time.Sleep(100 * time.Millisecond)
				readyCache.Update(testServiceKey, []*discov1.EndpointSlice{testEndpointSlice})
			}()
		} else {
			// Pre-populate: WaitForReady returns immediately with isColdStart=false
			readyCache.Update(testServiceKey, []*discov1.EndpointSlice{testEndpointSlice})
		}
	}

	// Create routing table
	routingTable := routingtest.NewTable()
	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-httpso",
			Namespace: "test-namespace",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "test-service",
				Port:    80,
			},
		},
	}
	routingTable.Memory[testHost] = ir

	// Build handler using production function
	var tlsCfg *tls.Config
	if cfg.tlsEnabled {
		tlsCfg = &tls.Config{InsecureSkipVerify: true}
	}
	handler := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       logr.Discard(),
		Queue:        fakeQueue,
		ReadyCache:   readyCache,
		RoutingTable: routingTable,
		Reader:       fake.NewClientBuilder().WithScheme(kedacache.NewScheme()).Build(),
		Timeouts: config.Timeouts{
			WorkloadReplicas:  5 * time.Second,
			ResponseHeader:    5 * time.Second,
			DisableKeepAlives: cfg.disableKeepAlives,
		},
		Serving:             config.Serving{EnableColdStartHeader: cfg.enableColdStartHeader},
		TLSConfig:           tlsCfg,
		Tracing:             config.Tracing{Enabled: cfg.tracingEnabled},
		dialAddressOverride: backend.Listener.Addr().String(),
	})

	return &proxyTestHarness{
		Handler:    handler,
		Backend:    backend,
		Queue:      fakeQueue,
		ReadyCache: readyCache,
	}
}

// doRequest sends a request through the proxy and returns the response.
func (h *proxyTestHarness) doRequest(t *testing.T, method, path, host string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	req.Host = host

	rec := httptest.NewRecorder()
	h.Handler.ServeHTTP(rec, req)

	return rec.Result()
}
