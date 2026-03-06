//go:build e2e

package path_prefix_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "path-prefix-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	deploymentName2      = fmt.Sprintf("%s-deployment-2", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	interceptorRouteName = fmt.Sprintf("%s-ir", testName)
	scaledObjectName     = fmt.Sprintf("%s-so", testName)
	host                 = testName
	pathPrefix0          = "/123/456"
	notPathPrefix0       = "/123/4567"
	pathPrefix1          = "/qwe/rty"
	notPathPrefix1       = "/qwe/rt"
	pathPrefix2          = "/qwe/oty"
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
	PathPrefix           string
	PathPrefix0          string
	PathPrefix1          string
	PathPrefix2          string
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
    - value: {{.PathPrefix0}}
    - value: {{.PathPrefix1}}
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
      targetValue: 20
  rules:
  - hosts:
    - {{.Host}}
    paths:
    - value: {{.PathPrefix2}}
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
        command: ["curl", "-H", "Host: {{.Host}}", "keda-add-ons-http-interceptor-proxy.keda:8080{{.PathPrefix}}"]
      restartPolicy: Never
  activeDeadlineSeconds: 600
  backoffLimit: 5
`
)

func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateNamespace(t, kc, testNamespace)

	t.Log("--- testing scale of workload1 with prefix0 ---")
	testScale(t, kc, data, templates, deploymentName, deploymentName2, pathPrefix0)
	t.Log("--- testing not scale ---")
	testNotScale(t, kc, data, templates, notPathPrefix0)
	t.Log("--- testing scale of workload1 with prefix1 ---")
	testScale(t, kc, data, templates, deploymentName, deploymentName2, pathPrefix1)
	t.Log("--- testing not scale ---")
	testNotScale(t, kc, data, templates, notPathPrefix1)
	t.Log("--- testing scale of workload2 with prefix2 ---")
	testScale(t, kc, data, templates, deploymentName2, deploymentName, pathPrefix2)

	// cleanup
	DeleteNamespace(t, testNamespace)
}

func testScale(t *testing.T, kc *kubernetes.Clientset, data templateData, templates []Template, scaledWorkload, notScaledWorkload, pathPrefix string) {
	// create resources
	KubectlApplyMultipleWithTemplate(t, data, templates)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	data.PathPrefix = pathPrefix

	// scale in
	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", maxReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	// scale out
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scaledWorkload, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, notScaledWorkload, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	// cleanup
	KubectlDeleteMultipleWithTemplate(t, data, templates)
}

func testNotScale(t *testing.T, kc *kubernetes.Clientset, data templateData, templates []Template, pathPrefix string) {
	// create resources
	KubectlApplyMultipleWithTemplate(t, data, templates)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName2, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	data.PathPrefix = pathPrefix

	// not scale
	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assertion := func(workload string, waitGroup *sync.WaitGroup) {
		AssertReplicaCountNotChangeDuringTimePeriod(t, kc, workload, testNamespace, minReplicaCount, 60)
		waitGroup.Done()
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go assertion(deploymentName, &wg)
	go assertion(deploymentName2, &wg)
	wg.Wait()

	// cleanup
	template := Template{Name: "loadJobTemplate", Config: loadJobTemplate}
	KubectlDeleteMultipleWithTemplate(t, data, append(templates, template))
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			InterceptorRouteName: interceptorRouteName,
			ScaledObjectName:     scaledObjectName,
			Host:                 host,
			PathPrefix0:          pathPrefix0,
			PathPrefix1:          pathPrefix1,
			PathPrefix2:          pathPrefix2,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
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
