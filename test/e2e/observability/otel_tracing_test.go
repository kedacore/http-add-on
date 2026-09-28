//go:build e2e

package observability_test

import (
	"context"
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
	jaegerNamespace      = "jaeger"
	jaegerService        = "jaeger"
	jaegerQueryPort      = "http-query"
	jaegerClientSpanKind = 3
)

type jaegerResponse struct {
	Result jaegerTraceData `json:"result"`
}

type jaegerTraceData struct {
	ResourceSpans []jaegerResourceSpans `json:"resourceSpans"`
}

type jaegerResourceSpans struct {
	ScopeSpans []jaegerScopeSpans `json:"scopeSpans"`
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

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			// Poll Jaeger for traces - they may take a moment to arrive.
			queryTime := time.Now().UTC()
			params := map[string]string{
				"query.serviceName":  "keda-http-interceptor",
				"query.searchDepth":  "100",
				"query.startTimeMin": queryTime.Add(-time.Hour).Format(time.RFC3339Nano),
				"query.startTimeMax": queryTime.Add(time.Hour).Format(time.RFC3339Nano),
			}

			var traces jaegerTraceData
			err := wait.For(func(_ context.Context) (bool, error) {
				body, err := f.ServiceProxyGet(jaegerNamespace, jaegerService, jaegerQueryPort, "/api/v3/traces", params)
				if err != nil {
					return false, nil
				}
				var jr jaegerResponse
				if err := json.Unmarshal(body, &jr); err != nil {
					return false, nil
				}
				traces = jr.Result
				return len(traces.ResourceSpans) > 0, nil
			}, wait.WithTimeout(2*time.Minute), wait.WithInterval(5*time.Second))
			if err != nil {
				t.Fatal("no traces found in Jaeger")
			}

			status := findSpanStatusCode(traces)
			if status != "200" {
				t.Errorf("expected span status code 200, got %q", status)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func findSpanStatusCode(traces jaegerTraceData) string {
	for _, resourceSpans := range traces.ResourceSpans {
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				if isClientSpanKind(span.Kind) {
					if status := getTagValue(span.Attributes, "http.response.status_code"); status != "" {
						return status
					}
				}
			}
		}
	}
	return ""
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
