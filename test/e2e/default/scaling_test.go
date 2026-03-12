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

func TestScaling(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("scaling").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("scaling-app")
			ir := f.CreateInterceptorRoute("scaling-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithRequestRate(5),
			)
			f.CreateScaledObject("scaling-so", app, ir, func(so *kedav1alpha1.ScaledObject) {
				so.Spec.MaxReplicaCount = ptr.To(int32(10))
			})

			return ctx
		}).
		Assess("starts at zero replicas", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForReplicas(app, 0)

			return ctx
		}).
		Assess("scales up under sustained load", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			// 10 req/s for 30s. With requestRate target of 5 req/s per replica,
			// this should stabilize at 2 replicas.
			stopLoad := f.GenerateLoad(
				url.URL{Host: f.Hostname()},
				vegeta.Rate{Freq: 10, Per: time.Second},
				30*time.Second,
			)

			f.WaitForReplicas(app, 2)

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
