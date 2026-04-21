//go:build e2e

package default_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestInvalidInterceptorRouteRejected(t *testing.T) {
	t.Parallel()

	noScalingMetric := features.New("no-scaling-metric").
		WithLabel("area", "validation").
		Assess("IR without any scaling metric is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			ir := &httpv1beta1.InterceptorRoute{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.keda.sh/v1beta1",
					Kind:       "InterceptorRoute",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-no-metric",
					Namespace: f.Namespace(),
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Service: "some-service",
						Port:    8080,
					},
					ScalingMetric: httpv1beta1.ScalingMetricSpec{},
				},
			}

			err := cfg.Client().Resources().Create(ctx, ir)
			if err == nil {
				t.Fatal("expected creation to fail for IR with no scaling metric, but it succeeded")
			}
			if !errors.IsInvalid(err) {
				t.Fatalf("expected Invalid error, got: %v", err)
			}

			return ctx
		}).
		Feature()

	bothPortAndPortName := features.New("both-port-and-portname").
		WithLabel("area", "validation").
		Assess("IR with both port and portName is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			ir := &httpv1beta1.InterceptorRoute{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "http.keda.sh/v1beta1",
					Kind:       "InterceptorRoute",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-both-ports",
					Namespace: f.Namespace(),
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Service:  "some-service",
						Port:     8080,
						PortName: "http",
					},
					ScalingMetric: httpv1beta1.ScalingMetricSpec{
						Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
							TargetValue: 100,
						},
					},
				},
			}

			err := cfg.Client().Resources().Create(ctx, ir)
			if err == nil {
				t.Fatal("expected creation to fail for IR with both port and portName, but it succeeded")
			}
			if !errors.IsInvalid(err) {
				t.Fatalf("expected Invalid error, got: %v", err)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, noScalingMetric, bothPortAndPortName)
}
