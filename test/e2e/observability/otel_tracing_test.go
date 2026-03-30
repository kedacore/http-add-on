//go:build e2e

package observability_test

import (
	"context"
	"encoding/json"
	"fmt"
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
)

type jaegerResponse struct {
	Data []jaegerTrace `json:"data"`
}

type jaegerTrace struct {
	TraceID string       `json:"traceID"`
	Spans   []jaegerSpan `json:"spans"`
}

type jaegerSpan struct {
	TraceID       string      `json:"traceID"`
	SpanID        string      `json:"spanID"`
	OperationName string      `json:"operationName"`
	Tags          []jaegerTag `json:"tags"`
}

type jaegerTag struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
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
			params := map[string]string{
				"service": "keda-http-interceptor",
				"limit":   "100",
			}

			var traces []jaegerTrace
			err := wait.For(func(_ context.Context) (bool, error) {
				body, err := f.ServiceProxyGet(jaegerNamespace, jaegerService, jaegerQueryPort, "/api/traces", params)
				if err != nil {
					return false, nil
				}
				var jr jaegerResponse
				if err := json.Unmarshal(body, &jr); err != nil {
					return false, nil
				}
				traces = jr.Data
				return len(traces) > 0, nil
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

func findSpanStatusCode(traces []jaegerTrace) string {
	for _, trace := range traces {
		for _, span := range trace.Spans {
			if getTagValue(span.Tags, "span.kind") == "client" {
				if status := getTagValue(span.Tags, "http.response.status_code"); status != "" {
					return status
				}
			}
		}
	}
	return ""
}

func getTagValue(tags []jaegerTag, key string) string {
	for _, tag := range tags {
		if tag.Key == key {
			return fmt.Sprintf("%v", tag.Value)
		}
	}
	return ""
}
