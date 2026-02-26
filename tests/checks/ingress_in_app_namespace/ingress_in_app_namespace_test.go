//go:build e2e

package ingress_in_app_namespace_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "ingress-in-app-namespace-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	ingressName          = fmt.Sprintf("%s-ingress", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	ingressHost          = fmt.Sprintf("http://%s-controller.%s", IngressReleaseName, IngressNamespace)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	IngressName          string
	KEDANamespace        string
	IngressHost          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
}

const (
	ingressTemplate = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{.IngressName}}
  namespace: {{.TestNamespace}}
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    kubernetes.io/ingress.class: nginx
spec:
  rules:
  - host: {{.Host}}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: {{.ServiceName}}-external
            port:
              number: 8080
`
	externalServiceTemplate = `
apiVersion: v1
kind: Service
metadata:
  name: {{.ServiceName}}-external
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  type: ExternalName
  externalName: keda-add-ons-http-interceptor-proxy.{{.KEDANamespace}}
`

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
        command: ["curl", "-H", "Host: {{.Host}}", "{{.IngressHost}}"]
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
	assert.True(t, WaitForIngressReady(t, kc, ingressName, testNamespace, 12, 10),
		"ingress should be ready after 2 minutes")

	testScaleOut(t, kc, data)
	testScaleIn(t, kc, data)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testScaleOut(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing scale out ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", maxReplicaCount)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing scale out ---")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			IngressName:          ingressName,
			KEDANamespace:        KEDANamespace,
			HTTPScaledObjectName: httpScaledObjectName,
			IngressHost:          ingressHost,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceTemplate", Config: serviceTemplate},
			{Name: "externalServiceTemplate", Config: externalServiceTemplate},
			{Name: "ingressTemplate", Config: ingressTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}
