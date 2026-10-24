//go:build e2e

package observability_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

const (
	jaegerNamespace = "jaeger"
	jaegerService   = "jaeger"
	jaegerQueryPort = "http-query"
	// OTLP SpanKind.CLIENT enum value.
	jaegerClientSpanKind = 3
)

type jaegerResponse struct {
	Result jaegerTraceData `json:"result"`
}

type jaegerTraceData struct {
	ResourceSpans []jaegerResourceSpans `json:"resourceSpans"`
}

type jaegerResourceSpans struct {
	Resource   jaegerResource     `json:"resource"`
	ScopeSpans []jaegerScopeSpans `json:"scopeSpans"`
}

type jaegerResource struct {
	Attributes []jaegerTag `json:"attributes"`
}

type jaegerScopeSpans struct {
	Spans []jaegerSpan `json:"spans"`
}

type jaegerSpan struct {
	Kind       any         `json:"kind"`
	Attributes []jaegerTag `json:"attributes"`
}

type jaegerTag struct {
	Key   string         `json:"key"`
	Value jaegerTagValue `json:"value"`
}

type jaegerTagValue struct {
	StringValue string `json:"stringValue"`
	IntValue    string `json:"intValue"`
}

func TestOtelTracing(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("otel-tracing").
		WithLabel("area", "observability").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("tracing-app")
			ir := f.CreateInterceptorRoute("tracing-ir", app, h.IRWithHosts(f.Hostname()))
			f.CreateScaledObject("tracing-so", app, ir)

			return ctx
		}).
		Assess("jaeger receives traces from interceptor", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			traceID, traceParent := newTraceparent(t)
			resp := f.ProxyRequest(h.Request{
				Host:    f.Hostname(),
				Headers: map[string]string{"traceparent": traceParent},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			// Poll Jaeger for this request's trace - it may take a moment to arrive.
			var clientSpanStatusCode string
			err := wait.For(func(_ context.Context) (bool, error) {
				path := "/api/v3/traces/" + traceID
				body, err := f.ServiceProxyGet(jaegerNamespace, jaegerService, jaegerQueryPort, path, nil)
				if err != nil {
					return false, nil
				}
				var jr jaegerResponse
				if err := json.Unmarshal(body, &jr); err != nil {
					return false, nil
				}
				var found bool
				clientSpanStatusCode, found = findInterceptorClientSpanStatusCode(jr.Result)
				return found, nil
			}, wait.WithTimeout(2*time.Minute), wait.WithInterval(5*time.Second))
			if err != nil {
				t.Fatal("no interceptor client span found in Jaeger")
			}

			if clientSpanStatusCode != "200" {
				t.Errorf("expected span status code 200, got %q", clientSpanStatusCode)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func newTraceparent(t *testing.T) (string, string) {
	t.Helper()

	var ids [24]byte
	if _, err := rand.Read(ids[:]); err != nil {
		t.Fatalf("generate trace context: %v", err)
	}

	traceID := hex.EncodeToString(ids[:16])
	parentSpanID := hex.EncodeToString(ids[16:])
	return traceID, "00-" + traceID + "-" + parentSpanID + "-01"
}

func findInterceptorClientSpanStatusCode(traces jaegerTraceData) (string, bool) {
	for _, resourceSpans := range traces.ResourceSpans {
		if getTagValue(resourceSpans.Resource.Attributes, "service.name") != "keda-http-interceptor" {
			continue
		}
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				if isClientSpanKind(span.Kind) {
					return getTagValue(span.Attributes, "http.response.status_code"), true
				}
			}
		}
	}
	return "", false
}

func isClientSpanKind(kind any) bool {
	switch kind := kind.(type) {
	case float64:
		return kind == jaegerClientSpanKind
	case string:
		return kind == "SPAN_KIND_CLIENT"
	default:
		return false
	}
}

func getTagValue(tags []jaegerTag, key string) string {
	for _, tag := range tags {
		if tag.Key == key {
			if tag.Value.StringValue != "" {
				return tag.Value.StringValue
			}
			return tag.Value.IntValue
		}
	}
	return ""
}
