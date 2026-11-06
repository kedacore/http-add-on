//go:build e2e

package default_test

import (
	"context"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestInterceptorScaledObject(t *testing.T) {
	t.Parallel()

	feat := features.New("interceptor-scaledobject").
		WithLabel("area", "scaling").
		Assess("is ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			err := wait.For(func(_ context.Context) (bool, error) {
				var so kedav1alpha1.ScaledObject
				if err := f.Get("keda-add-ons-http-interceptor", "keda", &so); err != nil {
					return false, nil
				}
				ready := so.Status.Conditions.GetReadyCondition()
				return ready.IsTrue(), nil
			}, wait.WithTimeout(time.Minute), wait.WithInterval(time.Second))
			if err != nil {
				t.Fatalf("timed out waiting for the interceptor ScaledObject to become ready: %v", err)
			}

			return ctx
		}).
		Assess("has an HPA", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			err := wait.For(func(_ context.Context) (bool, error) {
				var hpa autoscalingv2.HorizontalPodAutoscaler
				return f.Get("keda-hpa-keda-add-ons-http-interceptor", "keda", &hpa) == nil, nil
			}, wait.WithTimeout(time.Minute), wait.WithInterval(time.Second))
			if err != nil {
				t.Fatalf("timed out waiting for the interceptor HPA: %v", err)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
