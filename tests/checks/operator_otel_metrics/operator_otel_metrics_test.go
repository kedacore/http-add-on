//go:build e2e
// +build e2e

package operator_otel_metrics_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	prommodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "operator-otel-metrics-test"
)

var (
	testNamespace          = fmt.Sprintf("%s-ns", testName)
	deploymentName         = fmt.Sprintf("%s-deployment", testName)
	serviceName            = fmt.Sprintf("%s-service", testName)
	clientName             = fmt.Sprintf("%s-client", testName)
	httpScaledObjectName   = fmt.Sprintf("%s-http-so", testName)
	host                   = testName
	minReplicaCount        = 0
	maxReplicaCount        = 1
	otelCollectorPromURL   = "http://opentelemetry-collector.open-telemetry-system:8889/metrics"
	otlpGrpcClientEndpoint = "http://opentelemetry-collector.open-telemetry-system:4317"
	otlpHTTPClientEndpoint = "http://opentelemetry-collector.open-telemetry-system:4318"
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	ClientName           string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
}

const (
	serviceTemplate = `
apiVersion: v1
kind: Service
metadata:
  name: {{.ServiceName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: {{.DeploymentName}}
`

	deploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.DeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.DeploymentName}}
  template:
    metadata:
      labels:
        app: {{.DeploymentName}}
    spec:
      containers:
        - name: {{.DeploymentName}}
          image: registry.k8s.io/e2e-test-images/agnhost:2.45
          args:
          - netexec
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /
              port: http
`

	loadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: generate-request
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: curl-client
        image: curlimages/curl
        imagePullPolicy: Always
        command: ["curl", "-H", "Host: {{.Host}}", "keda-add-ons-http-interceptor-proxy.keda:8080"]
      restartPolicy: Never
  activeDeadlineSeconds: 600
  backoffLimit: 5
`

	httpScaledObjectTemplate = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.Host}}
  targetPendingRequests: 100
  scaledownPeriod: 10
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`

	clientTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}
spec:
  containers:
  - name: {{.ClientName}}
    image: curlimages/curl
    command:
      - sh
      - -c
      - "exec tail -f /dev/null"`
)

func TestMetricGeneration(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	// Send a test request to the interceptor
	sendLoad(t, kc, data)

	// Fetch metrics and validate them
	family := fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl --insecure %s", otelCollectorPromURL))
	val, ok := family["keda_http_scaled_object_total"]
	// If the metric is not found first time around then retry with a delay.
	if !ok {
		// Add a small sleep to allow metrics to be pushed from the exporter to the collector
		time.Sleep(10 * time.Second)
		// Fetch metrics and validate them
		family := fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl --insecure %s", otelCollectorPromURL))
		val, ok = family["keda_http_scaled_object_total"]
	}
	assert.True(t, ok, "keda_http_scaled_object_total is available")

	httpSacaledObjectCount := getMetricsValue(val)
	assert.GreaterOrEqual(t, httpSacaledObjectCount, float64(1))

	// Set the operator to comunicate over GRPC and test functionality
	changeOtlpProtocolInOperator(t, kc, "keda-add-ons-http-operator", "keda")
	CreateManyHttpScaledObjecs(t, 10)
	time.Sleep(time.Second * 10)
	family = fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl --insecure %s", otelCollectorPromURL))
	val, ok = family["keda_http_scaled_object_total"]
	assert.True(t, ok, "keda_http_scaled_object_total is available")

	httpSacaledObjectCountGrpc := getMetricsValue(val)
	assert.GreaterOrEqual(t, httpSacaledObjectCountGrpc, float64(10))

	DeleteManyHttpScaledObjecs(t, 10)
	time.Sleep(time.Second * 10)
	// Fetch metrics and validate them after deleting httpscaledobjects
	family = fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl --insecure %s", otelCollectorPromURL))
	val, ok = family["keda_http_scaled_object_total"]
	assert.True(t, ok, "keda_http_scaled_object_total is available")

	httpSacaledObjectCountAfterCleanUp := getMetricsValue(val)
	assert.Equal(t, float64(1), httpSacaledObjectCountAfterCleanUp)

	// cleanup
	fallbackHTTPProtocolInOperator(t, kc, "keda-add-ons-http-operator", "keda")
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func sendLoad(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- sending load ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)
}

func fetchAndParsePrometheusMetrics(t *testing.T, cmd string) map[string]*prommodel.MetricFamily {
	out, _, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, cmd)
	assert.NoErrorf(t, err, "cannot execute command - %s", err)

	parser := expfmt.TextParser{}
	// Ensure EOL
	reader := strings.NewReader(strings.ReplaceAll(out, "\r\n", "\n"))
	families, err := parser.TextToMetricFamilies(reader)
	assert.NoErrorf(t, err, "cannot parse metrics - %s", err)

	return families
}

func getMetricsValue(val *prommodel.MetricFamily) float64 {
	if val.GetName() == "keda_http_scaled_object_total" {
		metrics := val.GetMetric()
		for _, metric := range metrics {
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "namespace" && *label.Value == testNamespace {
					return metric.GetGauge().GetValue()
				}
			}
		}
	}
	return 0
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			ClientName:           clientName,
			HTTPScaledObjectName: httpScaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "clientTemplate", Config: clientTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}

func changeOtlpProtocolInOperator(t *testing.T, kc *kubernetes.Clientset, name string, namespace string) {
	operator, _ := kc.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	// Modify the environment variables
	t.Log("changeOtlpProtocolInOperator")
	for i, container := range operator.Spec.Template.Spec.Containers {
		if container.Name == name {
			container.Env = slices.DeleteFunc(container.Env, func(n corev1.EnvVar) bool {
				return n.Name == "OTEL_EXPORTER_OTLP_ENDPOINT"
			})

			container.Env = append(container.Env, corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "grpc"})
			container.Env = append(container.Env, corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: otlpGrpcClientEndpoint})
			operator.Spec.Template.Spec.Containers[i].Env = container.Env
		}
	}

	_, err := kc.AppsV1().Deployments(namespace).Update(context.TODO(), operator, metav1.UpdateOptions{})

	require.NoErrorf(t, err, "error changing keda http addon operator - %s", err)
	WaitForDeploymentReplicaReadyCount(t, kc, operator.Name, "keda", 1, 60, 2)
}

func fallbackHTTPProtocolInOperator(t *testing.T, kc *kubernetes.Clientset, name string, namespace string) {
	t.Log("fallback HTTP OTLP protocol in operator")

	operator, _ := kc.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	// Modify the environment variables
	for i, container := range operator.Spec.Template.Spec.Containers {
		if container.Name == name {
			container.Env = slices.DeleteFunc(container.Env, func(n corev1.EnvVar) bool {
				if n.Name == "OTEL_EXPORTER_OTLP_ENDPOINT" || n.Name == "OTEL_EXPORTER_OTLP_PROTOCOL" {
					return true
				}
				return false
			})
			container.Env = append(container.Env, corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: otlpHTTPClientEndpoint})
			operator.Spec.Template.Spec.Containers[i].Env = container.Env
		}
	}

	_, err := kc.AppsV1().Deployments(namespace).Update(context.TODO(), operator, metav1.UpdateOptions{})

	require.NoErrorf(t, err, "error changing keda http addon operator - %s", err)
	WaitForDeploymentReplicaReadyCount(t, kc, operator.Name, "keda", 1, 60, 2)
}

func getTemplateHTTPScaledObjecData(httpScaledObjecID string) (templateData, []Template) {
	deploymentCustomTemplateName := fmt.Sprintf("deploymentTemplate-%s", httpScaledObjecID)
	deploymentCustom := fmt.Sprintf("other-deployment-%s", httpScaledObjecID)
	httpScaledObjectCustom := fmt.Sprintf("other-http-scaled-object-name-%s", httpScaledObjecID)
	templateName := fmt.Sprintf("otherHttpScaledObjectName-%s", httpScaledObjecID)
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentCustom,
			ServiceName:          serviceName,
			ClientName:           clientName,
			HTTPScaledObjectName: httpScaledObjectCustom,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: templateName, Config: httpScaledObjectTemplate},
			{Name: deploymentCustomTemplateName, Config: deploymentTemplate},
		}
}

func CreateManyHttpScaledObjecs(t *testing.T, objectsCount int) {
	for i := 0; i < objectsCount; i++ {
		httpScaledObjecData, httpScaledObjecDataTemplates := getTemplateHTTPScaledObjecData(fmt.Sprintf("%d", i))
		KubectlApplyMultipleWithTemplate(t, httpScaledObjecData, httpScaledObjecDataTemplates)
	}
}
func DeleteManyHttpScaledObjecs(t *testing.T, objectsCount int) {
	for i := 0; i < objectsCount; i++ {
		httpScaledObjecData, httpScaledObjecDataTemplates := getTemplateHTTPScaledObjecData(fmt.Sprintf("%d", i))
		KubectlDeleteMultipleWithTemplate(t, httpScaledObjecData, httpScaledObjecDataTemplates)
	}
}
