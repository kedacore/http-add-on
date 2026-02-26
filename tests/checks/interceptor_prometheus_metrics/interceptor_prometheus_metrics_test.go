//go:build e2e

package interceptor_prometheus_metrics_test

import (
	"fmt"
	"strings"
	"testing"

	prommodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "interceptor-prom-metrics-test"
)

var (
	testNamespace                = fmt.Sprintf("%s-ns", testName)
	deploymentName               = fmt.Sprintf("%s-deployment", testName)
	serviceName                  = fmt.Sprintf("%s-service", testName)
	clientName                   = fmt.Sprintf("%s-client", testName)
	httpScaledObjectName         = fmt.Sprintf("%s-http-so", testName)
	host                         = testName
	minReplicaCount              = 0
	maxReplicaCount              = 1
	kedaInterceptorPrometheusURL = "http://keda-add-ons-http-interceptor-metrics.keda:2223/metrics"
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
	family := fetchAndParsePrometheusMetrics(t, fmt.Sprintf("curl --insecure %s", kedaInterceptorPrometheusURL))
	val, ok := family["interceptor_request_count_total"]
	assert.True(t, ok, "interceptor_request_count_total is available")

	requestCount := getMetricsValue(val)
	assert.GreaterOrEqual(t, requestCount, float64(1))

	// cleanup
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
	require.NoErrorf(t, err, "cannot execute command - %s", err)

	parser := expfmt.NewTextParser(model.UTF8Validation)
	// Ensure EOL
	reader := strings.NewReader(strings.ReplaceAll(out, "\r\n", "\n"))
	families, err := parser.TextToMetricFamilies(reader)
	require.NoErrorf(t, err, "cannot parse metrics - %s", err)

	return families
}

func getMetricsValue(val *prommodel.MetricFamily) float64 {
	if val.GetName() == "interceptor_request_count_total" {
		metrics := val.GetMetric()
		for _, metric := range metrics {
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "host" && *label.Value == testName {
					return metric.GetCounter().GetValue()
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
