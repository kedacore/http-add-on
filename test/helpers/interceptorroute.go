//go:build e2e

package helpers

import (
	"maps"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

// IROption configures an InterceptorRoute before creation.
type IROption func(*httpv1beta1.InterceptorRoute)

// CreateInterceptorRoute creates an InterceptorRoute targeting the given app in the cluster.
func (f *Framework) CreateInterceptorRoute(name string, app *TestApp, opts ...IROption) *httpv1beta1.InterceptorRoute {
	f.t.Helper()
	ir := &httpv1beta1.InterceptorRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "http.keda.sh/v1beta1",
			Kind:       "InterceptorRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.namespace,
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: app.Name,
				Port:    app.Port,
			},
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 100,
				},
			},
		},
	}
	for _, opt := range opts {
		opt(ir)
	}
	f.createResource(ir)
	// The interceptor picks up new routes asynchronously via its informer.
	// Sleep to give it time to load the route before tests send requests.
	time.Sleep(5 * time.Second)
	return ir
}

// IRWithHosts is a convenience for IRWithRules with a single host-based rule.
func IRWithHosts(hosts ...string) IROption {
	return IRWithRules(httpv1beta1.RoutingRule{Hosts: hosts})
}

// IRWithRules appends routing rules to the InterceptorRoute.
func IRWithRules(rules ...httpv1beta1.RoutingRule) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		ir.Spec.Rules = append(ir.Spec.Rules, rules...)
	}
}

// IRWithConcurrency sets the concurrency scaling metric target value.
func IRWithConcurrency(target int32) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		ir.Spec.ScalingMetric.Concurrency = &httpv1beta1.ConcurrencyTargetSpec{
			TargetValue: target,
		}
	}
}

// IRWithRequestRate sets the request rate scaling metric, clearing the concurrency metric.
// Uses a short 10s window so the rate decays quickly after load stops, enabling fast downscaling in tests.
func IRWithRequestRate(targetValue int32) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		ir.Spec.ScalingMetric.Concurrency = nil
		ir.Spec.ScalingMetric.RequestRate = &httpv1beta1.RequestRateTargetSpec{
			TargetValue: targetValue,
			Window:      metav1.Duration{Duration: 10 * time.Second},
		}
	}
}

// IRWithPortName overrides the target to use a named port instead of a numeric port.
func IRWithPortName(portName string) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		ir.Spec.Target.Port = 0
		ir.Spec.Target.PortName = portName
	}
}

// IRWithAnnotations sets annotations on the InterceptorRoute.
func IRWithAnnotations(annotations map[string]string) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		if ir.Annotations == nil {
			ir.Annotations = make(map[string]string)
		}
		maps.Copy(ir.Annotations, annotations)
	}
}

// IRWithColdStart configures a fallback service for cold-start scenarios.
func IRWithColdStart(fallbackService string, fallbackPort int32) IROption {
	return func(ir *httpv1beta1.InterceptorRoute) {
		ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
			Fallback: &httpv1beta1.TargetRef{
				Service: fallbackService,
				Port:    fallbackPort,
			},
		}
	}
}

// UpdateInterceptorRoute applies the given options to the IR and updates it.
func (f *Framework) UpdateInterceptorRoute(ir *httpv1beta1.InterceptorRoute, opts ...IROption) {
	f.t.Helper()
	// Re-fetch to get the latest resourceVersion before updating.
	if err := f.client.Resources().Get(f.ctx, ir.Name, ir.Namespace, ir); err != nil {
		f.t.Fatalf("failed to get InterceptorRoute %s/%s: %v", ir.Namespace, ir.Name, err)
	}
	for _, opt := range opts {
		opt(ir)
	}
	f.updateResource(ir)
	// The interceptor picks up route changes asynchronously via its informer.
	// Sleep to give it time to reload the route before tests send requests.
	time.Sleep(5 * time.Second)
}
