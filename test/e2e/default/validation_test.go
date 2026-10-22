//go:build e2e

package default_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	invalidSessionPersistence := features.New("invalid-session-persistence").
		WithLabel("area", "validation").
		Assess("IR with invalid sessionPersistence is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			tests := map[string]map[string]any{
				"empty":                     {},
				"missing type":              {"cookie": map[string]any{"name": "my-session"}},
				"unknown type":              {"type": "Header"},
				"empty cookie":              {"type": "Cookie", "cookie": map[string]any{}},
				"empty cookie name":         {"type": "Cookie", "cookie": map[string]any{"name": ""}},
				"cookie name with space":    {"type": "Cookie", "cookie": map[string]any{"name": "my session"}},
				"cookie name with ;":        {"type": "Cookie", "cookie": map[string]any{"name": "my;session"}},
				"cookie name too long":      {"type": "Cookie", "cookie": map[string]any{"name": strings.Repeat("a", 257)}},
				"absoluteTimeout below 1s":  {"type": "Cookie", "absoluteTimeout": "500ms"},
				"zero absoluteTimeout":      {"type": "Cookie", "absoluteTimeout": "0s"},
				"negative absoluteTimeout":  {"type": "Cookie", "absoluteTimeout": "-1m"},
				"malformed absoluteTimeout": {"type": "Cookie", "absoluteTimeout": "soon"},
			}

			i := 0
			for name, sp := range tests {
				i++
				t.Run(name, func(t *testing.T) {
					ir := newUnstructuredIRWithSessionPersistence(t, fmt.Sprintf("invalid-session-%d", i), f.Namespace(), sp)
					err := cfg.Client().Resources().Create(ctx, ir)
					if err == nil {
						t.Fatal("expected creation to fail, but it succeeded")
					}
					if !errors.IsInvalid(err) {
						t.Fatalf("expected Invalid error, got: %v", err)
					}
				})
			}

			return ctx
		}).
		Assess("IR with valid sessionPersistence is accepted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			tests := map[string]map[string]any{
				"type only":                {"type": "Cookie"},
				"token punctuation":        {"type": "Cookie", "cookie": map[string]any{"name": "a!#$%&'*+-.^_`|~9"}},
				"max length cookie name":   {"type": "Cookie", "cookie": map[string]any{"name": strings.Repeat("a", 256)}},
				"minimum absoluteTimeout":  {"type": "Cookie", "absoluteTimeout": "1s"},
				"compound absoluteTimeout": {"type": "Cookie", "absoluteTimeout": "1h30m"},
			}

			i := 0
			for name, sp := range tests {
				i++
				t.Run(name, func(t *testing.T) {
					ir := newUnstructuredIRWithSessionPersistence(t, fmt.Sprintf("valid-session-%d", i), f.Namespace(), sp)
					if err := cfg.Client().Resources().Create(ctx, ir); err != nil {
						t.Fatalf("expected creation to succeed, got: %v", err)
					}
				})
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, noScalingMetric, bothPortAndPortName, invalidSessionPersistence)
}

// newUnstructuredIRWithSessionPersistence builds a valid InterceptorRoute with
// the given raw sessionPersistence, bypassing typed serialization so malformed
// values reach the API server.
func newUnstructuredIRWithSessionPersistence(t *testing.T, name, namespace string, sessionPersistence map[string]any) *unstructured.Unstructured {
	t.Helper()

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&httpv1beta1.InterceptorRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "http.keda.sh/v1beta1",
			Kind:       "InterceptorRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "some-service",
				Port:    8080,
			},
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 100,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to convert InterceptorRoute: %v", err)
	}
	ir := &unstructured.Unstructured{Object: obj}
	if err := unstructured.SetNestedField(ir.Object, sessionPersistence, "spec", "sessionPersistence"); err != nil {
		t.Fatalf("failed to set sessionPersistence: %v", err)
	}
	return ir
}
