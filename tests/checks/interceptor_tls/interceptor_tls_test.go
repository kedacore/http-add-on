//go:build e2e

package interceptor_tls_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "interceptor-tls-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	clientName           = fmt.Sprintf("%s-client", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
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
    - port: 8443
      targetPort: https
      protocol: TCP
      name: https
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
          - --http-port
          - "8443"
          - --tls-cert-file
          - /certs/tls.crt
          - --tls-private-key-file
          - /certs/tls.key
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
            - name: https
              containerPort: 8443
              protocol: TCP
          volumeMounts:
            - readOnly: true
              mountPath: "/certs"
              name: certs
          readinessProbe:
            httpGet:
              path: /
              port: https
              scheme: HTTPS
      volumes:
        - name: certs
          secret:
            secretName: test-tls
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
    port: 8443
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

func TestInterceptorTLS(t *testing.T) {
	// setup
	t.Log("--- setting up ---")

	// create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	// setup certs
	_, err := ExecuteCommand(fmt.Sprintf("kubectl -n %s create secret tls test-tls --cert ../../../certs/tls.crt --key ../../../certs/tls.key", testNamespace))
	require.NoErrorf(t, err, "could not create tls cert secret in %s namespace - %s", testNamespace, err)

	// wait for test pod to start
	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, testNamespace, 10, 2),
		"test client count should be available after 20 seconds")

	// send test request and validate response body
	sendRequest(t)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func sendRequest(t *testing.T) {
	t.Log("--- sending request ---")

	stdout, _, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, fmt.Sprintf("curl -k -H 'Host: %s' https://keda-add-ons-http-interceptor-proxy.keda:8443/echo?msg=tls_test", host))
	require.NoErrorf(t, err, "could not run command on test client pod - %s", err)

	assert.Equal(t, "tls_test", stdout, "incorrect response body from test request: expected %s, got %s", "tls_test", stdout)
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
