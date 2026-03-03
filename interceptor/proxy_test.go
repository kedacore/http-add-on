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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/queue"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

const (
	testHost      = "test.example.com"
	testHTTPSOKey = "test-namespace/test-httpso"
)

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
			if got := resp.Header.Get("X-KEDA-HTTP-Cold-Start"); got != tt.wantHeader {
				t.Errorf("X-KEDA-HTTP-Cold-Start = %q, want %q", got, tt.wantHeader)
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

			hasResponseTraceparent := resp.Header.Get("Traceparent") != ""
			if hasResponseTraceparent != tt.tracingEnabled {
				t.Errorf("response has Traceparent = %v, want %v", hasResponseTraceparent, tt.tracingEnabled)
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
		if event.Host != testHTTPSOKey {
			t.Errorf("expected host %s, got %s", testHTTPSOKey, event.Host)
		}
		if event.Count != 1 {
			t.Errorf("expected +1, got %d", event.Count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for queue increment")
	}

	// Now unblock the waitFunc
	close(h.WaitCh)

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
	concurrency := counts.Counts[testHTTPSOKey].Concurrency
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
	Handler http.Handler
	Backend *httptest.Server
	Queue   *queue.FakeCounter
	WaitCh  chan struct{}
}

type harnessConfig struct {
	backendHandler        http.Handler
	enableColdStartHeader bool
	simulateColdStart     bool
	tlsEnabled            bool
	tracingEnabled        bool
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

	// Create queue and wait function
	var queueCounter queue.Counter
	var fakeQueue *queue.FakeCounter
	var waitCh chan struct{}
	var waitFunc func(context.Context, string, string) (bool, error)

	if cfg.useBlockingQueue {
		// Blocking mode: for testing queue counting behavior
		fakeQueue = queue.NewFakeCounter()
		queueCounter = fakeQueue
		waitCh = make(chan struct{})
		waitFunc = func(ctx context.Context, _, _ string) (bool, error) {
			select {
			case <-waitCh:
				return cfg.simulateColdStart, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
	} else {
		// Non-blocking mode: requests flow through immediately
		queueCounter = queue.NewFakeCounterBuffered()
		waitFunc = func(ctx context.Context, _, _ string) (bool, error) {
			return cfg.simulateColdStart, nil
		}
	}

	// Create routing table
	routingTable := routingtest.NewTable()
	httpso := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-httpso",
			Namespace: "test-namespace",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Name:    "test-deployment",
				Service: "test-service",
				Port:    80,
			},
		},
	}
	routingTable.Memory[testHost] = httpso

	// Build handler using production function
	var tlsCfg *tls.Config
	if cfg.tlsEnabled {
		tlsCfg = &tls.Config{InsecureSkipVerify: true}
	}
	handler := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       logr.Discard(),
		Queue:        queueCounter,
		WaitFunc:     waitFunc,
		RoutingTable: routingTable,
		Reader:       fake.NewClientBuilder().WithScheme(kedacache.NewScheme()).Build(),
		Timeouts: config.Timeouts{
			WorkloadReplicas: 5 * time.Second,
			ResponseHeader:   5 * time.Second,
		},
		Serving:             config.Serving{EnableColdStartHeader: cfg.enableColdStartHeader},
		TLSConfig:           tlsCfg,
		Tracing:             config.Tracing{Enabled: cfg.tracingEnabled},
		dialAddressOverride: backend.Listener.Addr().String(),
	})

	return &proxyTestHarness{
		Handler: handler,
		Backend: backend,
		Queue:   fakeQueue, // nil if not using blocking queue
		WaitCh:  waitCh,
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
