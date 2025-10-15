//go:build e2e
// +build e2e

package interceptor_otel_tracing_test

import (
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
	otelCollectorZipKinURL = "http://zipkin.zipkin:9411/api/v2/traces?serviceName=keda-http-interceptor\\&server.address=interceptor-otel-tracing-test\\&limit=1000"
	traces                 = Trace{}
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

type Trace [][]struct {
	TraceID       string `json:"traceId"`
	ParentID      string `json:"parentId"`
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Timestamp     int    `json:"timestamp"`
	Duration      int    `json:"duration"`
	LocalEndpoint struct {
		ServiceName string `json:"serviceName"`
	} `json:"localEndpoint"`
	Tags struct {
		HTTPFlavor                string `json:"http.flavor"`
		HTTPMethod                string `json:"http.method"`
		HTTPResponseContentLength string `json:"http.response_content_length"`
		HTTPStatusCode            string `json:"http.response.status_code"`
		HTTPURL                   string `json:"http.url"`
		HTTPUserAgent             string `json:"http.user_agent"`
		NetPeerName               string `json:"net.peer.name"`
		OtelLibraryName           string `json:"otel.library.name"`
		OtelLibraryVersion        string `json:"otel.library.version"`
		TelemetrySdkLanguage      string `json:"telemetry.sdk.language"`
		TelemetrySdkName          string `json:"telemetry.sdk.name"`
		TelemetrySdkVersion       string `json:"telemetry.sdk.version"`
	} `json:"tags"`
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

func TestTraceGeneration(t *testing.T) {
	t.Skip("Skipping test for now, as it is flaky. Will enable it again after investigation and fix.")
	// setup
	gomega.RegisterTestingT(t)
	t.Log("--- setting up ---")
	defer func() {
		kc := GetKubernetesClient(t)
		logs, err := FindPodLogs(kc, testNamespace, "job-name=generate-request")
		t.Log("--- printing logs for load job ---")
		t.Log(strings.Join(logs, "\n"))
		t.Logf("Error: %v", err)

		logs, err = FindPodLogs(kc, "zipkin", "app=zipkin")
		t.Log("--- printing logs for Zipkin ---")
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

	// Fetch metrics and validate them
	gomega.Eventually(func() int {
		traces = fetchAndParseZipkinTraces(t, fmt.Sprintf("curl %s", otelCollectorZipKinURL))
		return len(traces)
	}, 5*time.Minute, 5*time.Second).Should(gomega.BeNumerically(">", 0), "there should be at least 1 trace")

	traceStatus := getTracesStatus(traces)
	assert.EqualValues(t, "200", traceStatus)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func sendLoad(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- sending load ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)
}

func fetchAndParseZipkinTraces(t *testing.T, cmd string) Trace {
	out, errOut, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, cmd)
	assert.NoErrorf(t, err, "cannot execute command - %s", err)
	assert.Empty(t, errOut, "cannot execute command - %s", errOut)

	var traces Trace

	e := json.Unmarshal([]byte(out), &traces)
	if e != nil {
		assert.NoErrorf(t, err, "JSON decode error! - %s", e)
		return nil
	}

	return traces
}

func getTracesStatus(traces Trace) string {
	for _, t := range traces {
		for _, t1 := range t {
			if t1.Kind == "CLIENT" {
				s := t1.Tags.HTTPStatusCode
				return s
			}
		}
	}

	return ""
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
