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
	interceptorRouteName = fmt.Sprintf("%s-ir", testName)
	scaledObjectName     = fmt.Sprintf("%s-so", testName)
	host                 = testName
	pathPrefix           = "/v1/api"
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	InterceptorRouteName string
	ScaledObjectName     string
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
    paths:
    - value: {{.PathPrefix}}
    headers:
    - name: X-Custom-Header
      value: foo
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

	interceptorRouteTemplate2 = `
kind: InterceptorRoute
apiVersion: http.keda.sh/v1beta1
metadata:
  name: {{.InterceptorRouteName}}-2
  namespace: {{.TestNamespace}}
spec:
  target:
    service: {{.ServiceName}}-2
    port: 8080
  scalingMetric:
    concurrency:
      targetValue: 100
  rules:
  - hosts:
    - {{.Host}}
    paths:
    - value: {{.PathPrefix}}
    headers:
    - name: X-Custom-Header
      value: bar
`

	scaledObjectTemplate2 = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObjectName}}-2
  namespace: {{.TestNamespace}}
spec:
  cooldownPeriod: 10
  maxReplicaCount: {{ .MaxReplicas }}
  minReplicaCount: {{ .MinReplicas }}
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{.DeploymentName}}-2
  triggers:
  - type: external-push
    metadata:
      interceptorRoute: {{.InterceptorRouteName}}-2
      scalerAddress: keda-add-ons-http-external-scaler.keda:9090
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
			InterceptorRouteName: interceptorRouteName,
			ScaledObjectName:     scaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
			PathPrefix:           pathPrefix,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "interceptorRouteTemplate", Config: interceptorRouteTemplate},
			{Name: "scaledObjectTemplate", Config: scaledObjectTemplate},
			{Name: "deploymentTemplate2", Config: deploymentTemplate2},
			{Name: "serviceNameTemplate2", Config: serviceTemplate2},
			{Name: "interceptorRouteTemplate2", Config: interceptorRouteTemplate2},
			{Name: "scaledObjectTemplate2", Config: scaledObjectTemplate2},
		}
}
