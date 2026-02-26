//go:build e2e

package headerstest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "headers-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	deploymentName2      = fmt.Sprintf("%s-deployment-2", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	pathPrefix           = "/v1/api"
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	CustomHeader         string
	PathPrefix           string
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

	httpScaledObjectTemplate = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.Host}}
  pathPrefixes:
  - {{.PathPrefix}}
  headers:
  - name: X-Custom-Header
    value: foo
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

	serviceTemplate2 = `
apiVersion: v1
kind: Service
metadata:
  name: {{.ServiceName}}-2
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}-2
spec:
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: {{.DeploymentName}}-2
`

	deploymentTemplate2 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.DeploymentName}}-2
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}-2
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.DeploymentName}}-2
  template:
    metadata:
      labels:
        app: {{.DeploymentName}}-2
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

	httpScaledObjectTemplate2 = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}-2
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.Host}}
  pathPrefixes:
  - {{.PathPrefix}}
  headers:
  - name: X-Custom-Header
    value: bar
  targetPendingRequests: 100
  scaledownPeriod: 10
  scaleTargetRef:
    name: {{.DeploymentName}}-2
    service: {{.ServiceName}}-2
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
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
        command: ["curl", "-H", "Host: {{.Host}}", "-H", "X-Custom-Header: {{.CustomHeader}}", "keda-add-ons-http-interceptor-proxy.keda:8080{{.PathPrefix}}"]
      restartPolicy: Never
  activeDeadlineSeconds: 600
  backoffLimit: 5
`
)

func TestCheckHeadersRouting(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateNamespace(t, kc, testNamespace)

	t.Log("--- testing scale of workload1 with header foo ---")
	testScale(t, kc, data, templates, deploymentName, deploymentName2, "foo")
	t.Log("--- testing scale of workload2 with header bar ---")
	testScale(t, kc, data, templates, deploymentName2, deploymentName, "bar")

	// cleanup
	DeleteNamespace(t, testNamespace)
}

func testScale(t *testing.T, kc *kubernetes.Clientset, data templateData, templates []Template, scaledWorkload, notScaledWorkload, customHeader string) {
	// create resources
	KubectlApplyMultipleWithTemplate(t, data, templates)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	data.CustomHeader = customHeader

	// scale out
	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", maxReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	// scale in
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	// cleanup
	KubectlDeleteMultipleWithTemplate(t, data, templates)
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
			PathPrefix:           pathPrefix,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
			{Name: "deploymentTemplate2", Config: deploymentTemplate2},
			{Name: "serviceNameTemplate2", Config: serviceTemplate2},
			{Name: "httpScaledObjectTemplate2", Config: httpScaledObjectTemplate2},
		}
}
