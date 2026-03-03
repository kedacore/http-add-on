//go:build e2e

package httproute_in_app_namespace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "httproute-in-app-namespace-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httprouteName        = fmt.Sprintf("%s-httproute", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	referenceGrantName   = fmt.Sprintf("%s-rg", testName)
	gatewayHostPattern   = "http://%v.envoy-gateway-system.svc.cluster.local"
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPRouteName        string
	ReferenceGrantName   string
	KEDANamespace        string
	GatewayHost          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
}

const (
	httprouteTemplate = `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{.HTTPRouteName}}
  namespace: {{.TestNamespace}}
spec:
  parentRefs:
    - name: eg
      namespace: envoy-gateway-system
  hostnames:
    - {{.Host}}
  rules:
    - backendRefs:
        - kind: Service
          name: keda-add-ons-http-interceptor-proxy
          namespace: keda
          port: 8080
      matches:
        - path:
            type: PathPrefix
            value: /
`

	referenceGrantTemplate = `
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: {{.ReferenceGrantName}}
  namespace: keda
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: {{.TestNamespace}}
  to:
  - group: ""
    kind: Service
    name: keda-add-ons-http-interceptor-proxy
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
        command: ["curl", "-H", "Host: {{.Host}}", "{{.GatewayHost}}"]
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

func TestCheckHTTPRoute(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	gc := GetGatewayClient(t)
	data, templates := getTemplateData(t)
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)
	assert.True(t, WaitForHTTPRouteAccepted(t, gc, httprouteName, testNamespace, 12, 10),
		"HTTPRoute should be accepted after 2 minutes")

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
	t.Log("--- testing scale in ---")

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData(t *testing.T) (templateData, []Template) {
	kc := GetKubernetesClient(t)
	services, err := kc.CoreV1().Services(EnvoyNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list services in %s namespace: %v", EnvoyNamespace, err)
	}
	gatewayHost := ""
	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			gatewayHost = fmt.Sprintf(gatewayHostPattern, svc.Name)
			break
		}
	}
	if gatewayHost == "" {
		t.Fatalf("failed to find gateway host, no LB service found in %s namespace", EnvoyNamespace)
	}
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			HTTPRouteName:        httprouteName,
			ReferenceGrantName:   referenceGrantName,
			KEDANamespace:        KEDANamespace,
			HTTPScaledObjectName: httpScaledObjectName,
			GatewayHost:          gatewayHost,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceTemplate", Config: serviceTemplate},
			{Name: "httprouteTemplate", Config: httprouteTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
			{Name: "referenceGrantTemplate", Config: referenceGrantTemplate},
		}
}
