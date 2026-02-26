//go:build e2e

package scaling_phase_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "scaling-phase-deployment-with-skip-so-creation-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-so", testName)
	scaledObjectName     = fmt.Sprintf("%s-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 4
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
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

	workloadTemplate = `
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
  name: load-generator
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
          - "20000"
          - "-c"
          - "1"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 600
  backoffLimit: 5
`
	httpScaledObjectTemplate = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}
  namespace: {{.TestNamespace}}
  annotations:
    httpscaledobject.keda.sh/skip-scaledobject-creation: "true"
spec:
  hosts:
  - {{.Host}}
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 2
      window: 1m
  scaledownPeriod: 0
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`
	scaledObjectTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  cooldownPeriod: 10
  idleReplicaCount: 0
  maxReplicaCount: {{ .MaxReplicas }}
  minReplicaCount: {{ .MaxReplicas }}
  pollingInterval: 30
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{.DeploymentName}}
  triggers:
  - type: external-push
    metadata:
      httpScaledObject: {{.HTTPScaledObjectName}}
      scalerAddress: keda-add-ons-http-external-scaler.keda:9090
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

	testScaleOut(t, kc, data)
	testScaleIn(t, kc)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testScaleOut(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing scale out ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 18, 10),
		"replica count should be %d after 3 minutes", maxReplicaCount)
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing scale out ---")

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			HTTPScaledObjectName: httpScaledObjectName,
			ScaledObjectName:     scaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "workloadTemplate", Config: workloadTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
			{Name: "scaledObjectTemplate", Config: scaledObjectTemplate},
		}
}
