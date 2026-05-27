//go:build e2e

package default_test

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestMetricUpdate(t *testing.T) {
	t.Parallel()

	var ir *httpv1beta1.InterceptorRoute

	feat := features.New("metric-update").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("metric-update-app")
			ir = f.CreateInterceptorRoute("metric-update-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithConcurrency(5),
			)
			f.CreateScaledObject("metric-update-so", app, ir)

			return ctx
		}).
		Assess("HPA has initial target value", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForHPAMetricTarget("metric-update-so", 5)

			return ctx
		}).
		Assess("HPA target updates after IR metric change", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.UpdateInterceptorRoute(ir, h.IRWithConcurrency(10))
			f.WaitForHPAMetricTarget("metric-update-so", 10)

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
