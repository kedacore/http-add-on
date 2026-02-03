//go:build e2e
// +build e2e

package placeholderpages_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

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
      Content-Type: "text/plain; charset=utf-8"
      X-Test-Header: "test-value"
    content: "{{ "{{" }} .ServiceName {{ "}}" }} is starting up..."
`

func TestPlaceholderPages(t *testing.T) {
	// Test content-agnostic placeholder responses (HTML, JSON, plain text)
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
	assert.True(t, WaitForPodCountInNamespace(t, kc, testNamespace, 1, 60, 2),
		"curl client pod should exist")
	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, testNamespace, 60, 2),
		"curl client pod should be running")

	// Test placeholder response
	testPlaceholderResponse(t, kc)

	// Test HTML placeholder with user-controlled content
	testHTMLPlaceholder(t, kc, data)

	// Test JSON placeholder for API communication
	testJSONPlaceholder(t, kc, data)

	// Test plain text placeholder
	testPlainTextPlaceholder(t, kc, data)

	// Test placeholder disabled (backward compatibility)
	testPlaceholderDisabled(t, kc, data)

	// Test timestamp template variable
	testTimestampTemplateVariable(t, kc, data)

	// Test request ID template variable
	testRequestIDTemplateVariable(t, kc, data)

	// Test transition from placeholder to real service
	testPlaceholderToRealServiceTransition(t, kc, data)
}

func testPlaceholderResponse(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing default placeholder response ---")

	// Make request through interceptor
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
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
	assert.Contains(t, stdout, "Content-Type: text/plain", "should have correct Content-Type")
	assert.Contains(t, stdout, "is starting up", "should have placeholder message")

	// Verify NO automatic script injection
	assert.NotContains(t, stdout, "checkServiceStatus", "should NOT have automatic script injection")
}

func testHTMLPlaceholder(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing HTML placeholder with user-controlled content ---")

	htmlTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-html
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-html.test
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
      Content-Type: "text/html; charset=utf-8"
    content: |
      <!DOCTYPE html>
      <html>
      <head>
        <title>Service Starting</title>
        <meta http-equiv="refresh" content="{{ "{{" }} .RefreshInterval {{ "}}" }}">
      </head>
      <body>
        <h1>{{ "{{" }} .ServiceName {{ "}}" }} is starting - custom HTML</h1>
        <p>This is a user-controlled HTML placeholder.</p>
      </body>
      </html>
`

	KubectlApplyWithTemplate(t, data, "html-placeholder", htmlTemplate)
	defer KubectlDeleteWithTemplate(t, data, "html-placeholder", htmlTemplate)

	// Make request to HTML placeholder
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-html.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("HTML placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("HTML placeholder stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")

	// Verify custom HTML content
	assert.Contains(t, stdout, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, stdout, "Content-Type: text/html", "should have HTML Content-Type")
	assert.Contains(t, stdout, "<!DOCTYPE html>", "should have HTML doctype")
	assert.Contains(t, stdout, "is starting - custom HTML", "should have custom HTML content")
	assert.Contains(t, stdout, "user-controlled HTML placeholder", "should have custom message")
	assert.Contains(t, stdout, `<meta http-equiv="refresh" content="5">`, "should have user-controlled meta refresh")

	// Verify NO automatic script injection
	assert.NotContains(t, stdout, "checkServiceStatus", "should NOT have automatic script injection")
}

func testJSONPlaceholder(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing JSON placeholder for API communication ---")

	jsonTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-json
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-json.test
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
    statusCode: 202
    refreshInterval: 10
    headers:
      Content-Type: "application/json"
      Retry-After: "10"
    content: |
      {
        "status": "warming_up",
        "service": "{{ "{{" }} .ServiceName {{ "}}" }}",
        "namespace": "{{ "{{" }} .Namespace {{ "}}" }}",
        "retry_after_seconds": {{ "{{" }} .RefreshInterval {{ "}}" }}
      }
`

	KubectlApplyWithTemplate(t, data, "json-placeholder", jsonTemplate)
	defer KubectlDeleteWithTemplate(t, data, "json-placeholder", jsonTemplate)

	// Make request to JSON placeholder
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-json.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("JSON placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("JSON placeholder stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")

	// Verify JSON response
	assert.Contains(t, stdout, "HTTP/1.1 202", "should return 202 Accepted status")
	assert.Contains(t, stdout, "Content-Type: application/json", "should have JSON Content-Type")
	assert.Contains(t, stdout, "Retry-After: 10", "should have Retry-After header")
	assert.Contains(t, stdout, `"status": "warming_up"`, "should have status field")
	assert.Contains(t, stdout, `"service":`, "should have service field")
	assert.Contains(t, stdout, `"retry_after_seconds": 10`, "should have retry_after_seconds field")

	// Verify it's valid JSON structure (contains braces)
	assert.Contains(t, stdout, "{", "should have JSON opening brace")
	assert.Contains(t, stdout, "}", "should have JSON closing brace")
}

func testPlainTextPlaceholder(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing plain text placeholder ---")

	textTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-text
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-text.test
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
    refreshInterval: 3
    headers:
      Content-Type: "text/plain; charset=utf-8"
    content: |
      {{ "{{" }} .ServiceName {{ "}}" }} is currently unavailable.

      The service is scaling up to handle your request.
      Please retry in {{ "{{" }} .RefreshInterval {{ "}}" }} seconds.

      Namespace: {{ "{{" }} .Namespace {{ "}}" }}
`

	KubectlApplyWithTemplate(t, data, "text-placeholder", textTemplate)
	defer KubectlDeleteWithTemplate(t, data, "text-placeholder", textTemplate)

	// Make request to plain text placeholder
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-text.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Plain text placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("Plain text placeholder stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")

	// Verify plain text response
	assert.Contains(t, stdout, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, stdout, "Content-Type: text/plain", "should have plain text Content-Type")
	assert.Contains(t, stdout, "is currently unavailable", "should have unavailable message")
	assert.Contains(t, stdout, "Please retry in 3 seconds", "should have retry message with interval")
	assert.Contains(t, stdout, "Namespace:", "should have namespace in output")

	// Verify it's plain text (no HTML tags)
	assert.NotContains(t, stdout, "<html>", "should NOT have HTML tags")
	assert.NotContains(t, stdout, "<body>", "should NOT have body tag")
}

func testPlaceholderDisabled(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing placeholder disabled (backward compatibility) ---")

	// HTTPScaledObject WITHOUT placeholderConfig - should NOT return placeholder header
	disabledTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-disabled
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-disabled.test
  scaleTargetRef:
    name: {{.TestName}}
    service: {{.TestName}}
    port: 80
  replicas:
    min: 0
    max: 10
  scaledownPeriod: 10
`

	KubectlApplyWithTemplate(t, data, "disabled-placeholder", disabledTemplate)
	defer KubectlDeleteWithTemplate(t, data, "disabled-placeholder", disabledTemplate)

	// Make request with short timeout - should either timeout or get gateway timeout
	// The key assertion is that there's NO X-Keda-Http-Placeholder-Served header
	curlCmd := fmt.Sprintf("curl -si --max-time 3 -H 'Host: %s-disabled.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, _, _ := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Disabled placeholder output: %s", stdout)

	// Verify NO placeholder header - this confirms backward compatibility
	// Without placeholderConfig, the interceptor should NOT serve a placeholder
	assert.NotContains(t, stdout, "X-Keda-Http-Placeholder-Served",
		"should NOT have placeholder header when placeholderConfig is not set")
}

func testTimestampTemplateVariable(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing timestamp template variable ---")

	timestampTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-timestamp
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-timestamp.test
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
      Content-Type: "application/json"
    content: |
      {
        "status": "warming_up",
        "timestamp": "{{ "{{" }} .Timestamp {{ "}}" }}"
      }
`

	KubectlApplyWithTemplate(t, data, "timestamp-placeholder", timestampTemplate)
	defer KubectlDeleteWithTemplate(t, data, "timestamp-placeholder", timestampTemplate)

	// Record time before request
	beforeRequest := time.Now().Add(-time.Second)

	// Make request to timestamp placeholder
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-timestamp.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Timestamp placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("Timestamp placeholder stderr: %s", stderr)
	}

	// Record time after request
	afterRequest := time.Now().Add(time.Second)

	assert.NoError(t, err, "curl command should succeed")
	assert.Contains(t, stdout, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, stdout, "X-Keda-Http-Placeholder-Served", "should have placeholder header")

	// Extract timestamp from response using regex for RFC3339 format
	// RFC3339 format: 2006-01-02T15:04:05Z07:00
	timestampRegex := regexp.MustCompile(`"timestamp":\s*"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+-][^"]*)"`)
	matches := timestampRegex.FindStringSubmatch(stdout)
	assert.NotEmpty(t, matches, "should contain a timestamp in RFC3339 format")

	if len(matches) > 1 {
		parsedTime, err := time.Parse(time.RFC3339, matches[1])
		assert.NoError(t, err, "timestamp should be valid RFC3339")
		assert.True(t, parsedTime.After(beforeRequest), "timestamp should be after request start")
		assert.True(t, parsedTime.Before(afterRequest), "timestamp should be before request end")
		t.Logf("Parsed timestamp: %v", parsedTime)
	}
}

func testRequestIDTemplateVariable(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing request ID template variable ---")

	requestIDTemplate := `
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-requestid
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-requestid.test
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
      Content-Type: "text/plain"
    content: "Request ID: {{ "{{" }} .RequestID {{ "}}" }}"
`

	KubectlApplyWithTemplate(t, data, "requestid-placeholder", requestIDTemplate)
	defer KubectlDeleteWithTemplate(t, data, "requestid-placeholder", requestIDTemplate)

	// Make request with custom X-Request-ID header
	testRequestID := "test-request-id-12345"
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-requestid.test' -H 'X-Request-ID: %s' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName, testRequestID)
	stdout, stderr, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Request ID placeholder output: %s", stdout)
	if stderr != "" {
		t.Logf("Request ID placeholder stderr: %s", stderr)
	}

	assert.NoError(t, err, "curl command should succeed")
	assert.Contains(t, stdout, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, stdout, "X-Keda-Http-Placeholder-Served", "should have placeholder header")
	assert.Contains(t, stdout, testRequestID, "should contain the custom request ID in response body")
	assert.Contains(t, stdout, "Request ID: "+testRequestID, "should have the exact request ID format")
}

func testPlaceholderToRealServiceTransition(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing transition from placeholder to real service ---")

	transitionTemplate := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.TestName}}-transition
  namespace: {{.TestNamespace}}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.TestName}}-transition
  template:
    metadata:
      labels:
        app: {{.TestName}}-transition
    spec:
      containers:
      - name: {{.TestName}}-transition
        image: registry.k8s.io/e2e-test-images/agnhost:2.45
        args: ["netexec"]
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: {{.TestName}}-transition
  namespace: {{.TestNamespace}}
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: {{.TestName}}-transition
---
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}-transition
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}-transition.test
  scaleTargetRef:
    name: {{.TestName}}-transition
    service: {{.TestName}}-transition
    port: 80
  replicas:
    min: 0
    max: 10
  scaledownPeriod: 300
  placeholderConfig:
    enabled: true
    statusCode: 503
    refreshInterval: 5
    headers:
      Content-Type: "text/plain"
    content: "Service is starting..."
`

	KubectlApplyWithTemplate(t, data, "transition-placeholder", transitionTemplate)
	defer KubectlDeleteWithTemplate(t, data, "transition-placeholder", transitionTemplate)

	// Step 1: Verify placeholder response while scaled to 0
	t.Log("Step 1: Verify placeholder response while scaled to 0")
	curlCmd := fmt.Sprintf("curl -si -H 'Host: %s-transition.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)
	stdout, _, err := ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Placeholder response (scaled to 0): %s", stdout)

	assert.NoError(t, err, "curl command should succeed")
	assert.Contains(t, stdout, "X-Keda-Http-Placeholder-Served", "should have placeholder header when scaled to 0")
	assert.Contains(t, stdout, "Service is starting", "should have placeholder content")

	// Step 2: Scale deployment to 1 replica
	t.Log("Step 2: Scaling deployment to 1 replica")
	KubectlApplyWithTemplate(t, data, "scale-up", fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s-transition
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s-transition
  template:
    metadata:
      labels:
        app: %s-transition
    spec:
      containers:
      - name: %s-transition
        image: registry.k8s.io/e2e-test-images/agnhost:2.45
        args: ["netexec"]
        ports:
        - containerPort: 8080
`, testName, testNamespace, testName, testName, testName))

	// Step 3: Wait for pod to be ready
	t.Log("Step 3: Waiting for deployment pod to be ready")
	assert.True(t, WaitForPodCountInNamespace(t, kc, testNamespace, 2, 120, 2),
		"should have 2 pods (curl + service)")
	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, testNamespace, 120, 2),
		"all pods should be running")

	// Give some time for the routing table to update
	time.Sleep(5 * time.Second)

	// Step 4: Verify real service response
	t.Log("Step 4: Verify real service response after scale up")
	stdout, _, err = ExecCommandOnSpecificPod(t, "curl-client", testNamespace, curlCmd)
	stdout = RemoveANSI(stdout)
	t.Logf("Real service response (scaled to 1): %s", stdout)

	assert.NoError(t, err, "curl command should succeed")
	assert.Contains(t, stdout, "HTTP/1.1 200", "should return 200 OK from real service")
	assert.NotContains(t, stdout, "X-Keda-Http-Placeholder-Served",
		"should NOT have placeholder header when real service is available")
}
