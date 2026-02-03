//go:build stress
// +build stress

package stress

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	sustainedTestName = "sustained-load-stress-test"
)

var (
	sustainedTestNamespace        = fmt.Sprintf("%s-ns", sustainedTestName)
	sustainedDeploymentName       = fmt.Sprintf("%s-deployment", sustainedTestName)
	sustainedServiceName          = fmt.Sprintf("%s-service", sustainedTestName)
	sustainedHTTPScaledObjectName = fmt.Sprintf("%s-http-so", sustainedTestName)
	sustainedHost                 = sustainedTestName
	sustainedMinReplicaCount      = 1
	sustainedMaxReplicaCount      = 15
)

type sustainedTemplateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
	Requests             string
	Concurrency          string
}

const (
	sustainedServiceTemplate = `
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

	sustainedWorkloadTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.DeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  replicas: 1
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

	sustainedLoadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: sustained-load-generator
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: apache-ab
        image: ghcr.io/kedacore/tests-apache-ab
        imagePullPolicy: Always
        args:
          - "-n"
          - "{{.Requests}}"
          - "-c"
          - "{{.Concurrency}}"
          - "-t"
          - "600"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 1800
  backoffLimit: 5
`
	sustainedHTTPScaledObjectTemplate = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.Host}}
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 50
      window: 1m
  scaledownPeriod: 30
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`
)

func TestSustainedLoadStress(t *testing.T) {
	// setup
	t.Log("--- setting up sustained load stress test ---")
	kc := GetKubernetesClient(t)
	data, templates := getSustainedTemplateData()
	CreateKubernetesResources(t, kc, sustainedTestNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, sustainedDeploymentName, sustainedTestNamespace, sustainedMinReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", sustainedMinReplicaCount)

	testSustainedLoad(t, kc, data)
	testSustainedScaleIn(t, kc)

	// cleanup
	DeleteKubernetesResources(t, sustainedTestNamespace, data, templates)
}

func testSustainedLoad(t *testing.T, kc *kubernetes.Clientset, data sustainedTemplateData) {
	t.Log("--- testing sustained load (10 minutes of continuous traffic with 50 concurrent connections) ---")

	KubectlApplyWithTemplate(t, data, "sustainedLoadJobTemplate", sustainedLoadJobTemplate)

	// Wait for initial scale up
	t.Log("--- waiting for initial scale up ---")
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, sustainedDeploymentName, sustainedTestNamespace, 5, 18, 10),
		"replica count should reach at least 5 after 3 minutes")

	// Verify the system continues to handle load and scales appropriately
	t.Log("--- verifying continued scaling and stability under sustained load ---")
	// The system should scale up more as load continues
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, sustainedDeploymentName, sustainedTestNamespace, 8, 30, 10),
		"replica count should reach at least 8 after 5 minutes of sustained load")

	// Let the load continue and verify system remains stable
	t.Log("--- verifying system stability for extended period ---")
	// System should maintain high replica count while under load
	assert.True(t, WaitForDeploymentReplicaReadyMinCount(t, kc, sustainedDeploymentName, sustainedTestNamespace, 5, 24, 10),
		"replica count should remain at least 5 during sustained load")

	KubectlDeleteWithTemplate(t, data, "sustainedLoadJobTemplate", sustainedLoadJobTemplate)
}

func testSustainedScaleIn(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing scale in after sustained stress test ---")

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, sustainedDeploymentName, sustainedTestNamespace, sustainedMinReplicaCount, 36, 10),
		"replica count should be %d after 6 minutes", sustainedMinReplicaCount)
}

func getSustainedTemplateData() (sustainedTemplateData, []Template) {
	return sustainedTemplateData{
			TestNamespace:        sustainedTestNamespace,
			DeploymentName:       sustainedDeploymentName,
			ServiceName:          sustainedServiceName,
			HTTPScaledObjectName: sustainedHTTPScaledObjectName,
			Host:                 sustainedHost,
			MinReplicas:          sustainedMinReplicaCount,
			MaxReplicas:          sustainedMaxReplicaCount,
			Requests:             "500000",
			Concurrency:          "50",
		}, []Template{
			{Name: "sustainedWorkloadTemplate", Config: sustainedWorkloadTemplate},
			{Name: "sustainedServiceNameTemplate", Config: sustainedServiceTemplate},
			{Name: "sustainedHTTPScaledObjectTemplate", Config: sustainedHTTPScaledObjectTemplate},
		}
}
