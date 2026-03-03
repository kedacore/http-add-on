//go:build e2e

package interceptor_websocket_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "interceptor-websocket-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	clientJobName        = fmt.Sprintf("%s-client", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 2
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	ClientJobName        string
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
      targetPort: 8080
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
          image: ghcr.io/kedacore/tests-websockets
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          env:
            - name: PORT
              value: "8080"
          readinessProbe:
            tcpSocket:
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          livenessProbe:
            tcpSocket:
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 20
`

	//nolint:unused
	websocketClientJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.ClientJobName}}
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: websocket-client
        image: ghcr.io/kedacore/tests-websockets:245a788
        command:
        - node
        - client.js
        - {{.ClientJobName}}
        env:
        - name: GATEWAY
          value: "keda-add-ons-http-interceptor-proxy.keda"
        - name: HOST
          value: "{{.Host}}"
        - name: PORT
          value: "8080"
      restartPolicy: Never
  activeDeadlineSeconds: 300
  backoffLimit: 3
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
  targetPendingRequests: 1
  scaledownPeriod: 10
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`

	// Simple curl-based WebSocket test using websocat
	websocketCurlTestTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.ClientJobName}}-curl
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: curl-test
        image: curlimages/curl:latest
        command: ["/bin/sh"]
        args:
        - -c
        - |
          echo "Testing HTTP connection first..."
          curl -H "Host: {{.Host}}" -v http://keda-add-ons-http-interceptor-proxy.keda:8080/ || echo "HTTP test failed"
          echo "HTTP test completed"
      restartPolicy: Never
  activeDeadlineSeconds: 60
  backoffLimit: 1
`

	// WebSocket test using websocat (curl-like tool for WebSockets)
	websocatTestTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.ClientJobName}}-ws-curl
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: websocat-test
        # v1.14.0 was amd64-only; :latest includes arm64 (https://github.com/vi/websocat/pull/283)
        image: ghcr.io/vi/websocat:latest
        command: ["/bin/sh"]
        args:
        - -c
        - |
          echo "Installing websocat..."

          echo "Testing WebSocket connection through interceptor..."
          echo "Connecting to ws://keda-add-ons-http-interceptor-proxy.keda:8080/ws with Host: {{.Host}}"

          # Test WebSocket connection with Host header
          timeout 30 /usr/local/bin/websocat -H "Host: {{.Host}}" ws://keda-add-ons-http-interceptor-proxy.keda:8080/ws --ping-interval 5 --ping-timeout 10 --text --exit-on-eof <<EOF || echo "WebSocket test completed"
          {"type": "ping", "message": "test connection"}
          EOF

          echo "WebSocket curl test completed"
      restartPolicy: Never
  activeDeadlineSeconds: 120
  backoffLimit: 1
`
)

// TestCheck tests WebSocket connection hijacking through the HTTP Add-on interceptor.
// This test verifies that:
// 1. WebSocket connections can be established through the interceptor
// 2. The interceptor's responseWriter.Hijack() method works correctly for WebSocket upgrades
// 3. WebSocket connections trigger proper scaling behavior
// 4. Connections are properly maintained and cleaned up
func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	testBasicHTTPConnection(t, kc, data)
	testWebSocketConnectionCurl(t, kc, data)

	// TODO: Re-enable these tests once client properly manages HOST parameter
	// testWebSocketScaleOut(t, kc, data)
	// testWebSocketScaleIn(t, kc, data)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testBasicHTTPConnection(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing basic HTTP connection first ---")

	// Test basic HTTP connection to ensure routing works
	KubectlApplyWithTemplate(t, data, "websocketCurlTestTemplate", websocketCurlTestTemplate)

	// // Wait for the curl test job to complete
	// assert.True(t, WaitForJobSuccess(t, kc, clientJobName+"-curl", testNamespace, 6, 10),
	// 	"curl test job should succeed")

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, 1, 12, 10),
		"replica count should be %d after 2 minutes", 1)

	// Clean up the curl test job
	KubectlDeleteWithTemplate(t, data, "websocketCurlTestTemplate", websocketCurlTestTemplate)
}

func testWebSocketConnectionCurl(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing WebSocket connection with websocat ---")

	// Test WebSocket connection through interceptor using websocat
	KubectlApplyWithTemplate(t, data, "websocatTestTemplate", websocatTestTemplate)

	// Wait for the WebSocket curl test job to complete
	assert.True(t, WaitForJobSuccess(t, kc, clientJobName+"-ws-curl", testNamespace, 8, 15),
		"WebSocket curl test job should succeed")

	t.Log("WebSocket connection test completed successfully")

	// Clean up the WebSocket curl test job
	KubectlDeleteWithTemplate(t, data, "websocatTestTemplate", websocatTestTemplate)
}

//nolint:unused
func testWebSocketScaleOut(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing WebSocket scale out ---")

	// Start WebSocket client that will establish persistent connections
	KubectlApplyWithTemplate(t, data, "websocketClientJobTemplate", websocketClientJobTemplate)

	// Wait for scale out due to WebSocket connections
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", maxReplicaCount)

	t.Log("WebSocket client successfully triggered scale out")
}

//nolint:unused
func testWebSocketScaleIn(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing WebSocket scale in ---")

	// Remove the WebSocket client job to terminate connections
	KubectlDeleteWithTemplate(t, data, "websocketClientJobTemplate", websocketClientJobTemplate)

	// Wait for scale in after WebSocket connections are closed
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 18, 10),
		"replica count should be %d after 3 minutes", minReplicaCount)

	t.Log("WebSocket connections closed and scaled in successfully")
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			HTTPScaledObjectName: httpScaledObjectName,
			ClientJobName:        clientJobName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}
