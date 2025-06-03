//go:build e2e
// +build e2e

package placeholder_pages

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName      = "placeholder-test"
	testNamespace = testName + "-ns"
)

type templateData struct {
	TestNamespace string
	TestName      string
}

const testTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.TestName}}
  namespace: {{.TestNamespace}}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.TestName}}
  template:
    metadata:
      labels:
        app: {{.TestName}}
    spec:
      containers:
      - name: {{.TestName}}
        image: registry.k8s.io/e2e-test-images/agnhost:2.45
        args: ["netexec"]
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: {{.TestName}}
  namespace: {{.TestNamespace}}
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: {{.TestName}}
---
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}.test
  scaleTargetRef:
    name: {{.TestName}}
    service: {{.TestName}}
    port: 80
  replicas:
    min: 0
    max: 10
  scaledownPeriod: 10
  placeholderConfig:
    enabled: true
    statusCode: 503
    refreshInterval: 5
    headers:
      X-Test-Header: "test-value"
`

func TestPlaceholderPages(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data := templateData{
		TestNamespace: testNamespace,
		TestName:      testName,
	}

	CreateNamespace(t, kc, testNamespace)
	defer DeleteNamespace(t, testNamespace)

	KubectlApplyWithTemplate(t, data, "placeholder-test", testTemplate)
	defer KubectlDeleteWithTemplate(t, data, "placeholder-test", testTemplate)

	// Create a test pod to make requests
	clientPod := `
apiVersion: v1
kind: Pod
metadata:
  name: curl-client
  namespace: ` + testNamespace + `
spec:
  containers:
  - name: curl
    image: curlimages/curl
    command: ["sleep", "3600"]
`
	// Create the pod using KubectlApplyWithTemplate
	KubectlApplyWithTemplate(t, data, "curl-client", clientPod)
	defer KubectlDeleteWithTemplate(t, data, "curl-client", clientPod)

	// Wait for curl pod to be ready
	assert.True(t, WaitForPodCountInNamespace(t, kc, testNamespace, 1, 30, 2),
		"curl client pod should be ready")

	// Give container time to fully initialize
	_, _ = ExecuteCommand("sleep 2")

	// Test placeholder response
	testPlaceholderResponse(t, kc)

	// Test custom placeholder with script injection
	testCustomPlaceholderWithScript(t, kc, data)
}

func testPlaceholderResponse(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing placeholder response ---")

	// Make request through interceptor
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	t.Logf("curl output: %s", stdout)
	if stderr != "" {
		t.Logf("curl stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")

	// Verify placeholder response
	assert.Contains(t, stdout, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, stdout, "X-Keda-Http-Placeholder-Served", "should have placeholder header")
	assert.Contains(t, stdout, "X-Test-Header", "should have custom header")
	assert.Contains(t, stdout, "test-value", "should have custom header value")
	assert.Contains(t, stdout, "is starting up", "should have placeholder message")
}

func testCustomPlaceholderWithScript(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing custom placeholder with script injection ---")

	// Create a custom HTTPScaledObject with inline content
	customTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-custom
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-custom.test
  scaleTargetRef:
    name: {{.TestName}}
    service: {{.TestName}}
    port: 80
  replicas:
    min: 0
    max: 10
  scaledownPeriod: 10
  placeholderConfig:
    enabled: true
    statusCode: 503
    refreshInterval: 7
    content: |
      <html>
        <body>
          <h1>Service is starting - custom page</h1>
          <p>This is a test placeholder page.</p>
        </body>
      </html>
`

	KubectlApplyWithTemplate(t, data, "custom-placeholder", customTemplate)
	defer KubectlDeleteWithTemplate(t, data, "custom-placeholder", customTemplate)

	// Make request to custom placeholder
	curlCmd := fmt.Sprintf("curl -s -H 'Host: %s-custom.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	t.Logf("custom placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("custom placeholder stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")

	// Verify custom content is there
	assert.Contains(t, stdout, "Service is starting - custom page", "should have custom placeholder content")
	assert.Contains(t, stdout, "This is a test placeholder page.", "should have custom message")

	// Verify script was injected
	assert.Contains(t, stdout, "checkServiceStatus", "should have injected checkServiceStatus function")
	assert.Contains(t, stdout, "X-KEDA-HTTP-Placeholder-Served", "should have header check in script")
	assert.Contains(t, stdout, "checkInterval =  7  * 1000", "should have correct refresh interval in script")
}
