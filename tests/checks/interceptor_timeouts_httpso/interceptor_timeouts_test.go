//go:build e2e

package interceptor_timeouts_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "interceptor-timeouts-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
	requestJobName       = fmt.Sprintf("%s-request", testName)
	responseDelay        = "0"
)

type templateData struct {
	TestNamespace         string
	DeploymentName        string
	ServiceName           string
	HTTPScaledObjectName  string
	ResponseHeaderTimeout string
	Host                  string
	MinReplicas           int
	MaxReplicas           int
	RequestJobName        string
	ResponseDelay         string
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
    - port: 9898
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
          image: stefanprodan/podinfo:latest
          ports:
            - name: http
              containerPort: 9898
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
`

	loadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.RequestJobName}}
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: curl-client
        image: curlimages/curl
        imagePullPolicy: Always
        command: ["curl", "-f", "-H", "Host: {{.Host}}", "keda-add-ons-http-interceptor-proxy.keda:8080/delay/{{.ResponseDelay}}"]
      restartPolicy: Never
  activeDeadlineSeconds: 600
  backoffLimit: 2
`

	httpScaledObjectWithoutTimeoutsTemplate = `
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
    port: 9898
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`

	httpScaledObjectWithTimeoutsTemplate = `
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
    port: 9898
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
  timeouts:
    responseHeader: "{{ .ResponseHeaderTimeout }}s"
`
)

func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	testDefaultTimeouts(t, kc, data)
	testCustomTimeouts(t, kc, data)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testDefaultTimeouts(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	KubectlApplyWithTemplate(t, data, "httpScaledObjectTemplate", httpScaledObjectWithoutTimeoutsTemplate)

	testDefaultTimeoutPasses(t, kc, data)
	testDefaultTimeoutFails(t, kc, data)

	KubectlDeleteWithTemplate(t, data, "httpScaledObjectTemplate", httpScaledObjectWithoutTimeoutsTemplate)
}

func testDefaultTimeoutPasses(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing default timeout passes ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)

	assert.True(t, WaitForJobSuccess(t, kc, requestJobName, testNamespace, 1, 1), "request should succeed")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func testDefaultTimeoutFails(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing default timeout fails ---")

	data.ResponseDelay = "2"

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)

	assert.False(t, WaitForJobSuccess(t, kc, requestJobName, testNamespace, 1, 1), "request should fail")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func testCustomTimeouts(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	data.ResponseHeaderTimeout = "5"

	KubectlApplyWithTemplate(t, data, "httpScaledObjectTemplate", httpScaledObjectWithTimeoutsTemplate)

	testCustomTimeoutPasses(t, kc, data)
	testCustomTimeoutFails(t, kc, data)

	KubectlDeleteWithTemplate(t, data, "httpScaledObjectTemplate", httpScaledObjectWithTimeoutsTemplate)
}

func testCustomTimeoutPasses(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing custom timeout passes ---")

	data.ResponseDelay = "2"

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)

	assert.True(t, WaitForJobSuccess(t, kc, requestJobName, testNamespace, 1, 1), "request should succeed")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func testCustomTimeoutFails(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing custom timeout fails ---")

	data.ResponseDelay = "7"

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)

	assert.False(t, WaitForJobSuccess(t, kc, requestJobName, testNamespace, 1, 1), "request should fail")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			HTTPScaledObjectName: httpScaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
			RequestJobName:       requestJobName,
			ResponseDelay:        responseDelay,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
		}
}
