//go:build e2e

package default_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestConcurrencyScaling(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("concurrency-scaling").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("concurrency-app")
			ir := f.CreateInterceptorRoute("concurrency-ir", app,
				h.IRWithHosts(f.Hostname()),
				// Each replica should handle 5 concurrent requests. With
				// sustained load that keeps ~10 connections open, we expect
				// around 2 replicas.
				h.IRWithConcurrency(5),
			)
			f.CreateScaledObject("concurrency-so", app, ir, func(so *kedav1alpha1.ScaledObject) {
				so.Spec.MaxReplicaCount = ptr.To(int32(10))
			})

			return ctx
		}).
		Assess("starts at zero replicas", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.WaitForReplicas(app, 0)
			return ctx
		}).
		Assess("scales up under sustained concurrent load", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			// Use a slow backend (wait=500ms per request) with 10 req/s.
			// At 10 req/s with 500ms latency, ~5 requests are in-flight
			// concurrently. With a concurrency target of 5 per replica,
			// this keeps the system just at the boundary so we expect at
			// least 1 replica (cold-start proven) and up to 2.
			stopLoad := f.GenerateLoad(
				url.URL{Host: f.Hostname(), RawQuery: "wait=500ms"},
				vegeta.Rate{Freq: 10, Per: time.Second},
				30*time.Second,
			)

			f.WaitForReplicas(app, 1)

			metrics := stopLoad()
			if metrics.Requests == 0 {
				t.Fatal("no requests were sent during load generation")
			}

			return ctx
		}).
		Assess("scales back to zero after load stops", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForReplicas(app, 0)
			f.AssertReplicasStable(app, 0, 30*time.Second)

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
