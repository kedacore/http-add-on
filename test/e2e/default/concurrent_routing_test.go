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

func TestConcurrentRouting(t *testing.T) {
	t.Parallel()

	var appA, appB *h.TestApp

	feat := features.New("concurrent-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appA = f.CreateTestApp("app-a")
			appB = f.CreateTestApp("app-b")

			irA := f.CreateInterceptorRoute("ir-a", appA,
				h.IRWithHosts(f.Hostname("app-a")),
				h.IRWithRequestRate(5),
			)
			irB := f.CreateInterceptorRoute("ir-b", appB,
				h.IRWithHosts(f.Hostname("app-b")),
				h.IRWithRequestRate(5),
			)

			f.CreateScaledObject("so-a", appA, irA, func(so *kedav1alpha1.ScaledObject) {
				so.Spec.MaxReplicaCount = ptr.To(int32(10))
			})
			f.CreateScaledObject("so-b", appB, irB, func(so *kedav1alpha1.ScaledObject) {
				so.Spec.MaxReplicaCount = ptr.To(int32(10))
			})

			return ctx
		}).
		Assess("both start at zero", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForReplicas(appA, 0)
			f.WaitForReplicas(appB, 0)

			return ctx
		}).
		Assess("load on app-a scales only app-a", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			stopLoad := f.GenerateLoad(
				url.URL{Host: f.Hostname("app-a")},
				vegeta.Rate{Freq: 10, Per: time.Second},
				30*time.Second,
			)

			f.WaitForMinReplicas(appA, 1)
			f.AssertReplicasStable(appB, 0, 10*time.Second)

			metrics := stopLoad()
			if metrics.Requests == 0 {
				t.Fatal("no requests were sent during load generation")
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
