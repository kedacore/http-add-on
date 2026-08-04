package handler

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/util"
)

// deadServerURL returns the URL of a server that was closed before serving,
// so dials to it are refused immediately.
func deadServerURL(t *testing.T) *url.URL {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	u, err := url.Parse("http://" + addr)
	require.NoError(t, err)
	return u
}

// staticPicker mimics the cache-backed endpoint picker: it returns altHost
// until altHost has been tried.
func staticPicker(altHost string) util.EndpointPickerFunc {
	return func(tried map[string]struct{}) string {
		if _, ok := tried[altHost]; ok {
			return ""
		}
		return altHost
	}
}

func TestUpstreamRetriesAlternateEndpointOnConnectFailure(t *testing.T) {
	r := require.New(t)

	var alternateHits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alternateHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("from alternate"))
		assert.NoError(t, err)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	r.NoError(err)

	deadURL := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 1, Instruments: metrics.NewNoopInstruments()},
	)

	res, req, err := reqAndRes("/test")
	r.NoError(err)
	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL)
	ctx = util.ContextWithEndpointPicker(ctx, staticPicker(backendURL.Host))
	req = req.WithContext(ctx)

	uh.ServeHTTP(res, req)

	r.Equal(http.StatusOK, res.Code, "response body: %s", res.Body.String())
	r.Equal("from alternate", res.Body.String())
	r.Equal(int32(1), alternateHits.Load())
}

func TestUpstreamRetryBudgetExhausted(t *testing.T) {
	r := require.New(t)

	deadURL1 := deadServerURL(t)
	deadURL2 := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 1, Instruments: metrics.NewNoopInstruments()},
	)

	res, req, err := reqAndRes("/test")
	r.NoError(err)
	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL1)
	ctx = util.ContextWithEndpointPicker(ctx, staticPicker(deadURL2.Host))
	req = req.WithContext(ctx)

	start := time.Now()
	uh.ServeHTTP(res, req)
	elapsed := time.Since(start)

	// Connection refused is not a timeout, so the failure maps to 502.
	r.Equal(http.StatusBadGateway, res.Code, "response body: %s", res.Body.String())
	// Two bounded attempts (3 dials each at a 50ms interval) must fail far
	// quicker than the legacy unbounded redial, which would run until the
	// 10s request timeout of defaultTimeouts.
	r.Less(elapsed, 5*time.Second)
}

func TestUpstreamDoesNotRetryRequestWithBody(t *testing.T) {
	r := require.New(t)

	var alternateHits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alternateHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	r.NoError(err)

	deadURL := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 1, Instruments: metrics.NewNoopInstruments()},
	)

	req, err := http.NewRequest("POST", "/test", strings.NewReader("payload"))
	r.NoError(err)
	res := httptest.NewRecorder()

	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL)
	ctx = util.ContextWithEndpointPicker(ctx, staticPicker(backendURL.Host))
	// Bound the legacy unbounded dial redial for the test.
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	uh.ServeHTTP(res, req)

	r.Equal(http.StatusGatewayTimeout, res.Code, "response body: %s", res.Body.String())
	r.Equal(int32(0), alternateHits.Load(), "requests with a body must not be retried")
}

func TestUpstreamRetryDisabled(t *testing.T) {
	r := require.New(t)

	var alternateHits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		alternateHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	r.NoError(err)

	deadURL := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 0, Instruments: metrics.NewNoopInstruments()},
	)

	res, req, err := reqAndRes("/test")
	r.NoError(err)
	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL)
	ctx = util.ContextWithEndpointPicker(ctx, staticPicker(backendURL.Host))
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	uh.ServeHTTP(res, req)

	r.Equal(http.StatusGatewayTimeout, res.Code, "response body: %s", res.Body.String())
	r.Equal(int32(0), alternateHits.Load(), "retry count 0 must disable retries")
}

// TestUpstreamSingleEndpointKeepsLegacyDialRetry verifies that when the
// snapshot has no alternate endpoint, the first attempt keeps the legacy
// context-bounded dial retry instead of failing fast — single-replica cold
// starts rely on it.
func TestUpstreamSingleEndpointKeepsLegacyDialRetry(t *testing.T) {
	r := require.New(t)

	deadURL := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 1, Instruments: metrics.NewNoopInstruments()},
	)

	res, req, err := reqAndRes("/test")
	r.NoError(err)
	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL)
	// No alternate available: the picker returns "".
	ctx = util.ContextWithEndpointPicker(ctx, func(map[string]struct{}) string { return "" })
	ctx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	start := time.Now()
	uh.ServeHTTP(res, req)
	elapsed := time.Since(start)

	r.Equal(http.StatusGatewayTimeout, res.Code, "response body: %s", res.Body.String())
	// A bounded first attempt would fail after ~150ms; the legacy redial must
	// run until the request context deadline.
	r.GreaterOrEqual(elapsed, 350*time.Millisecond)
}

func TestUpstreamRetryMetric(t *testing.T) {
	r := require.New(t)

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	instruments, err := metrics.NewInstruments(provider)
	r.NoError(err)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	r.NoError(err)

	deadURL := deadServerURL(t)

	timeouts := defaultTimeouts()
	uh := NewUpstream(
		newTestTransport(retryDialContextFunc(timeouts)),
		newFakeClient(),
		config.Tracing{},
		timeouts.ResponseHeader,
		RetryConfig{Count: 1, Instruments: instruments},
	)

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ir", Namespace: "test-ns"},
	}

	res, req, err := reqAndRes("/test")
	r.NoError(err)
	ctx := util.ContextWithUpstreamURL(req.Context(), deadURL)
	ctx = util.ContextWithEndpointPicker(ctx, staticPicker(backendURL.Host))
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	req = req.WithContext(ctx)

	uh.ServeHTTP(res, req)
	r.Equal(http.StatusOK, res.Code, "response body: %s", res.Body.String())

	var rm metricdata.ResourceMetrics
	r.NoError(reader.Collect(context.Background(), &rm))

	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != metrics.MetricRequestRetries {
				continue
			}
			found = true
			sum, ok := m.Data.(metricdata.Sum[int64])
			r.True(ok, "retry metric should be a Sum[int64]")
			r.Len(sum.DataPoints, 1)
			dp := sum.DataPoints[0]
			r.Equal(int64(1), dp.Value)
			outcome, _ := dp.Attributes.Value(attribute.Key(metrics.AttrOutcome))
			r.Equal(metrics.OutcomeSuccess, outcome.AsString())
			routeName, _ := dp.Attributes.Value(attribute.Key(metrics.AttrRouteName))
			r.Equal("test-ir", routeName.AsString())
			routeNs, _ := dp.Attributes.Value(attribute.Key(metrics.AttrRouteNamespace))
			r.Equal("test-ns", routeNs.AsString())
		}
	}
	r.True(found, "retry metric %q not found", metrics.MetricRequestRetries)
}
