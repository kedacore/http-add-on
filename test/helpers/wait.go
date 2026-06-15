//go:build e2e

package helpers

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

const (
	defaultWaitTimeout  = 2 * time.Minute
	defaultPollInterval = 2 * time.Second
)

// WaitForReplicas polls until the app's deployment has exactly the expected number of ready replicas.
func (f *Framework) WaitForReplicas(app *TestApp, expected int32) {
	f.t.Helper()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	err := wait.For(
		conditions.New(f.client.Resources()).ResourceMatch(dep, func(object k8s.Object) bool {
			d := object.(*appsv1.Deployment)
			return d.Status.ReadyReplicas == expected
		}),
		wait.WithTimeout(defaultWaitTimeout),
	)
	if err != nil {
		f.t.Fatalf("timed out waiting for %s/%s to reach %d replicas: %v",
			app.Namespace, app.Name, expected, err)
	}
}

// WaitForMinReplicas polls until the app's deployment has at least the given number of ready replicas.
func (f *Framework) WaitForMinReplicas(app *TestApp, minReplicas int32) {
	f.t.Helper()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}

	err := wait.For(
		conditions.New(f.client.Resources()).ResourceMatch(dep, func(object k8s.Object) bool {
			d := object.(*appsv1.Deployment)
			return d.Status.ReadyReplicas >= minReplicas
		}),
		wait.WithTimeout(defaultWaitTimeout),
	)
	if err != nil {
		f.t.Fatalf("timed out waiting for %s/%s to reach at least %d replicas: %v",
			app.Namespace, app.Name, minReplicas, err)
	}
}

// WaitForHPAMetricTarget polls the HPA created by KEDA for the given
// ScaledObject until every external metric has the expected AverageValue.
func (f *Framework) WaitForHPAMetricTarget(soName string, expected int32) {
	f.t.Helper()

	hpaName := "keda-hpa-" + soName
	want := resource.NewMilliQuantity(int64(expected)*1000, resource.DecimalSI)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: f.namespace,
		},
	}

	err := wait.For(
		conditions.New(f.client.Resources()).ResourceMatch(hpa, func(object k8s.Object) bool {
			h := object.(*autoscalingv2.HorizontalPodAutoscaler)
			found := false
			for _, m := range h.Spec.Metrics {
				if m.Type != autoscalingv2.ExternalMetricSourceType || m.External == nil {
					continue
				}
				found = true
				if m.External.Target.AverageValue == nil || m.External.Target.AverageValue.Cmp(*want) != 0 {
					return false
				}
			}
			return found
		}),
		wait.WithTimeout(defaultWaitTimeout),
	)
	if err != nil {
		f.t.Fatalf("timed out waiting for HPA %s/%s to have metric target %d: %v",
			f.namespace, hpaName, expected, err)
	}
}

// AssertReplicasStable asserts that the deployment maintains exactly the expected replica count for the given duration.
func (f *Framework) AssertReplicasStable(app *TestApp, expected int32, duration time.Duration) {
	f.t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		dep := &appsv1.Deployment{}
		if err := f.client.Resources().Get(f.ctx, app.Name, app.Namespace, dep); err != nil {
			f.t.Fatalf("failed to get deployment %s/%s: %v", app.Namespace, app.Name, err)
		}

		if dep.Status.ReadyReplicas != expected {
			f.t.Fatalf("replica count changed: expected %d, got %d", expected, dep.Status.ReadyReplicas)
		}

		time.Sleep(defaultPollInterval)
	}
}
