//go:build e2e

package default_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestInterceptorRouteHostAdmission(t *testing.T) {
	t.Parallel()

	feat := features.New("host-admission").
		WithLabel("area", "validation").
		Assess("invalid hosts are rejected on create and update", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			resources := cfg.Client().Resources()
			for _, static := range []bool{false, true} {
				ir := &httpv1beta1.InterceptorRoute{
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("host-validation-%t", static), Namespace: f.Namespace()},
					Spec: httpv1beta1.InterceptorRouteSpec{
						Target: httpv1beta1.TargetRef{Service: "some-service", Port: 8080},
						ScalingMetric: httpv1beta1.ScalingMetricSpec{
							Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 1},
						},
					},
				}
				if err := resources.Create(ctx, ir); err != nil {
					t.Fatal(err)
				}
				for _, tt := range []struct {
					host  string
					valid bool
				}{
					{"", true},
					{"*", true},
					{"*.Example.COM:8080", true},
					{"2001:db8::1", true},
					{"[2001:db8::1]:443", true},
					{"[]", false},
					{"[]:80", false},
					{":80", false},
					{"api*.example.com", false},
					{"[not-an-ip]:80", false},
					{"example.com/path", false},
					{"example.com:65536", false},
				} {
					t.Run(fmt.Sprintf("static=%t/host=%s", static, tt.host), func(t *testing.T) {
						candidate := ir.DeepCopy()
						candidate.ResourceVersion = ""
						candidate.Name += "-create"
						rules := []httpv1beta1.RoutingRule{{Hosts: []string{tt.host}}}
						spec := map[string]any{}
						if static {
							candidate.Spec.StaticRoutes = []httpv1beta1.StaticRoute{{Rules: rules, Response: httpv1beta1.StaticResponse{StatusCode: 200}}}
							spec["staticRoutes"] = candidate.Spec.StaticRoutes
						} else {
							candidate.Spec.Rules = rules
							spec["rules"] = rules
						}
						err := resources.Create(ctx, candidate, func(o *metav1.CreateOptions) { o.DryRun = []string{metav1.DryRunAll} })
						if tt.valid && err != nil {
							t.Fatalf("create valid host: %v", err)
						}
						if !tt.valid && !errors.IsInvalid(err) {
							t.Fatalf("create invalid host: expected Invalid, got %v", err)
						}
						data, err := json.Marshal(map[string]any{"spec": spec})
						if err != nil {
							t.Fatal(err)
						}
						err = resources.Patch(ctx, ir.DeepCopy(), k8s.Patch{PatchType: types.MergePatchType, Data: data},
							func(o *metav1.PatchOptions) { o.DryRun = []string{metav1.DryRunAll} })
						if tt.valid && err != nil {
							t.Fatalf("update valid host: %v", err)
						}
						if !tt.valid && !errors.IsInvalid(err) {
							t.Fatalf("update invalid host: expected Invalid, got %v", err)
						}
					})
				}
			}
			return ctx
		}).Feature()
	testenv.Test(t, feat)
}
