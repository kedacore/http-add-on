//go:build e2e

package helpers

import (
	"bytes"
	"context"
	"io"
	"testing"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/yaml"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

// dumpDiagnostics dumps diagnostic information for the HTTP Addon and the test namespace.
func dumpDiagnostics(ctx context.Context, t *testing.T, client klient.Client) {
	t.Helper()

	clientset, err := kubernetes.NewForConfig(client.RESTConfig())
	if err != nil {
		t.Logf("diagnostics: failed to create clientset: %v", err)
		return
	}

	dumpPodLogs(ctx, t, client, clientset, AddonNamespace)

	testNS := namespaceFromContext(ctx)
	dumpPodLogs(ctx, t, client, clientset, testNS)
	dumpEvents(ctx, t, client, testNS)
	dumpDeployments(ctx, t, client, testNS)
	dumpServices(ctx, t, client, testNS)
	dumpInterceptorRoutes(ctx, t, client, testNS)
	dumpScaledObjects(ctx, t, client, testNS)
}

func dumpPodLogs(ctx context.Context, t *testing.T, client klient.Client, clientset *kubernetes.Clientset, namespace string) {
	t.Helper()

	var pods corev1.PodList
	if err := client.Resources(namespace).List(ctx, &pods); err != nil {
		t.Logf("diagnostics: failed to list pods in %s: %v", namespace, err)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: container.Name,
				TailLines: ptr.To(int64(100)),
			})
			stream, err := req.Stream(ctx)
			if err != nil {
				t.Logf("diagnostics: failed to get logs for %s/%s/%s: %v", namespace, pod.Name, container.Name, err)
				continue
			}
			var buf bytes.Buffer
			_, copyErr := io.Copy(&buf, stream)
			_ = stream.Close()
			if copyErr != nil {
				t.Logf("diagnostics: failed to copy logs for %s/%s/%s: %v", namespace, pod.Name, container.Name, copyErr)
			}

			t.Logf("=== Logs: %s/%s/%s ===\n%s", namespace, pod.Name, container.Name, buf.String())
		}
	}
}

func dumpEvents(ctx context.Context, t *testing.T, client klient.Client, namespace string) {
	t.Helper()

	var events corev1.EventList
	if err := client.Resources(namespace).List(ctx, &events); err != nil {
		t.Logf("diagnostics: failed to list events in %s: %v", namespace, err)
		return
	}

	t.Logf("=== Events: %s (%d) ===", namespace, len(events.Items))
	for _, e := range events.Items {
		ts := e.LastTimestamp.Time
		if ts.IsZero() {
			ts = e.EventTime.Time
		}
		t.Logf("  %s [%s] %s/%s: %s (reason=%s, count=%d)",
			ts.Format("15:04:05"), e.Type, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Message, e.Reason, e.Count)
	}
}

func dumpDeployments(ctx context.Context, t *testing.T, client klient.Client, namespace string) {
	t.Helper()

	var deps appsv1.DeploymentList
	if err := client.Resources(namespace).List(ctx, &deps); err != nil {
		t.Logf("diagnostics: failed to list deployments in %s: %v", namespace, err)
		return
	}

	for _, dep := range deps.Items {
		logResourceYAML(t, "Deployment", namespace, dep.Name, &dep)
	}
}

func dumpServices(ctx context.Context, t *testing.T, client klient.Client, namespace string) {
	t.Helper()

	var svcs corev1.ServiceList
	if err := client.Resources(namespace).List(ctx, &svcs); err != nil {
		t.Logf("diagnostics: failed to list services in %s: %v", namespace, err)
		return
	}

	for _, svc := range svcs.Items {
		logResourceYAML(t, "Service", namespace, svc.Name, &svc)
	}
}

func dumpInterceptorRoutes(ctx context.Context, t *testing.T, client klient.Client, namespace string) {
	t.Helper()

	var list httpv1beta1.InterceptorRouteList
	if err := client.Resources(namespace).List(ctx, &list); err != nil {
		t.Logf("diagnostics: failed to list interceptorroutes in %s: %v", namespace, err)
		return
	}

	for _, ir := range list.Items {
		logResourceYAML(t, "InterceptorRoute", namespace, ir.Name, &ir)
	}
}

func dumpScaledObjects(ctx context.Context, t *testing.T, client klient.Client, namespace string) {
	t.Helper()

	var list kedav1alpha1.ScaledObjectList
	if err := client.Resources(namespace).List(ctx, &list); err != nil {
		t.Logf("diagnostics: failed to list scaledobjects in %s: %v", namespace, err)
		return
	}

	for _, so := range list.Items {
		logResourceYAML(t, "ScaledObject", namespace, so.Name, &so)
	}
}

func logResourceYAML(t *testing.T, kind, namespace, name string, obj client.Object) {
	t.Helper()

	// unset managedFields for less verbose output
	obj.SetManagedFields(nil)

	out, err := yaml.Marshal(obj)
	if err != nil {
		t.Logf("diagnostics: failed to marshal %s %s/%s: %v", kind, namespace, name, err)
		return
	}
	t.Logf("=== %s: %s/%s ===\n%s", kind, namespace, name, string(out))
}
