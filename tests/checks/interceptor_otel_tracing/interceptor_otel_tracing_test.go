//go:build e2e

package interceptor_otel_tracing_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "interceptor-otel-tracing-test"

	jaegerNamespace   = "jaeger"
	jaegerServiceName = "jaeger"
	jaegerServicePort = "query"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	interceptorRouteName = fmt.Sprintf("%s-ir", testName)
	scaledObjectName     = fmt.Sprintf("%s-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

// Jaeger HTTP API response types. Defined here because the upstream types in
// the jaeger repo are in an internal package and cannot be imported.
type jaegerResponse struct {
	Data []jaegerTrace `json:"data"`
}

type jaegerTrace struct {
	TraceID string       `json:"traceID"`
	Spans   []jaegerSpan `json:"spans"`
}

type jaegerSpan struct {
	TraceID       string      `json:"traceID"`
	SpanID        string      `json:"spanID"`
	OperationName string      `json:"operationName"`
	Tags          []jaegerTag `json:"tags"`
}

type jaegerTag struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	InterceptorRouteName string
	ScaledObjectName     string
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
  ttlSecondsAfterFinished: 0
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

	interceptorRouteTemplate = `
kind: InterceptorRoute
apiVersion: http.keda.sh/v1beta1
metadata:
  name: {{.InterceptorRouteName}}
  namespace: {{.TestNamespace}}
spec:
  target:
    service: {{.ServiceName}}
    port: 8080
  scalingMetric:
    concurrency:
      targetValue: 100
  rules:
  - hosts:
    - {{.Host}}
`

	scaledObjectTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  cooldownPeriod: 10
  maxReplicaCount: {{ .MaxReplicas }}
  minReplicaCount: {{ .MinReplicas }}
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{.DeploymentName}}
  triggers:
  - type: external-push
    metadata:
      interceptorRoute: {{.InterceptorRouteName}}
      scalerAddress: keda-add-ons-http-external-scaler.keda:9090
`
)

func TestTraceGeneration(t *testing.T) {
	// setup
	gomega.RegisterTestingT(t)
	t.Log("--- setting up ---")
	defer func() {
		kc := GetKubernetesClient(t)
		logs, err := FindPodLogs(kc, testNamespace, "job-name=generate-request")
		t.Log("--- printing logs for load job ---")
		t.Log(strings.Join(logs, "\n"))
		t.Logf("Error: %v", err)

		logs, err = FindPodLogs(kc, jaegerNamespace, "app=jaeger")
		t.Log("--- printing logs for Jaeger ---")
		t.Log(strings.Join(logs, "\n"))
		t.Logf("Error: %v", err)

		logs, err = FindPodLogs(kc, "open-telemetry-system", "app.kubernetes.io/name=opentelemetry-collector")
		t.Log("--- printing logs for OTel ---")
		t.Log(strings.Join(logs, "\n"))
		t.Logf("Error: %v", err)
	}()

	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	// Send a test request to the interceptor
	sendLoad(t, kc, data)

	// Poll Jaeger for traces via K8s API proxy
	var traces []jaegerTrace
	gomega.Eventually(func() int {
		traces = queryJaegerTraces(t, kc)
		return len(traces)
	}, 5*time.Minute, 5*time.Second).Should(gomega.BeNumerically(">", 0), "there should be at least 1 trace")
	t.Logf("found %d trace(s) from Jaeger", len(traces))

	traceStatus := findSpanStatusCode(traces)
	assert.Equal(t, "200", traceStatus)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func sendLoad(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Helper()
	t.Log("--- sending load ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)
}

// queryJaegerTraces queries the Jaeger HTTP API via the Kubernetes API server's
// built-in service proxy, avoiding the need for port-forwarding or in-cluster
// curl pods.
func queryJaegerTraces(t *testing.T, kc *kubernetes.Clientset) []jaegerTrace {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := kc.CoreV1().Services(jaegerNamespace).
		ProxyGet("http", jaegerServiceName, jaegerServicePort, "/api/traces", map[string]string{
			"service": "keda-http-interceptor",
			"limit":   "100",
		}).
		DoRaw(ctx)
	if err != nil {
		t.Logf("error querying jaeger via k8s proxy: %v", err)
		return nil
	}

	var resp jaegerResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Logf("failed to parse jaeger response: %v, body: %s", err, string(data))
		return nil
	}
	return resp.Data
}

// findSpanStatusCode returns the HTTP response status code from the first
// client span found in the traces.
func findSpanStatusCode(traces []jaegerTrace) string {
	for _, trace := range traces {
		for _, span := range trace.Spans {
			if getTagValue(span.Tags, "span.kind") == "client" {
				if status := getTagValue(span.Tags, "http.response.status_code"); status != "" {
					return status
				}
			}
		}
	}
	return ""
}

// getTagValue returns the string representation of the first tag matching key,
// or an empty string if not found.
func getTagValue(tags []jaegerTag, key string) string {
	for _, tag := range tags {
		if tag.Key == key {
			return fmt.Sprintf("%v", tag.Value)
		}
	}
	return ""
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			InterceptorRouteName: interceptorRouteName,
			ScaledObjectName:     scaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "interceptorRouteTemplate", Config: interceptorRouteTemplate},
			{Name: "scaledObjectTemplate", Config: scaledObjectTemplate},
		}
}
