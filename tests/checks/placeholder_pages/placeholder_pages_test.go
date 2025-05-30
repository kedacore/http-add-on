//go:build e2e
// +build e2e

package placeholder_pages

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName        = "placeholder-pages-test"
	testServiceName = testName
	testNamespace   = testName + "-ns"
)

type placeholderTemplateData struct {
	TestNamespace   string
	TestName        string
	TestServiceName string
}

const placeholderTemplate = `
apiVersion: v1
kind: Namespace
metadata:
  name: {{.TestNamespace}}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.TestName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.TestName}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{.TestName}}
  template:
    metadata:
      labels:
        app: {{.TestName}}
    spec:
      containers:
      - name: app
        image: kennethreitz/httpbin:latest
        ports:
        - containerPort: 80
          name: http
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  name: {{.TestServiceName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.TestName}}
spec:
  ports:
  - port: 80
    targetPort: 80
    name: http
  selector:
    app: {{.TestName}}
  type: ClusterIP
---
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: {{.TestName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.TestName}}
  pathPrefixes:
  - /
  scaleTargetRef:
    service: {{.TestServiceName}}
    port: 80
    deployment: {{.TestName}}
  targetPendingRequests: 1
  scaledownPeriod: 60
  placeholderConfig:
    enabled: true
    statusCode: 503
    refreshInterval: 5
    headers:
      X-Service-Status: "warming-up"
`

func TestPlaceholderPages(t *testing.T) {
	// Create test data
	data := placeholderTemplateData{
		TestNamespace:   testNamespace,
		TestName:        testName,
		TestServiceName: testServiceName,
	}

	// Apply the resources
	KubectlApplyWithTemplate(t, data, "placeholder-test", placeholderTemplate)

	// Ensure cleanup
	defer func() {
		KubectlDeleteWithTemplate(t, data, "placeholder-test", placeholderTemplate)
		DeleteNamespace(t, testNamespace)
	}()

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, testName, testNamespace, 1, 240, 3),
		"deployment replicas should be 1 after 3 minutes")

	// Wait for the deployment to scale down to zero
	time.Sleep(90 * time.Second)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, testName, testNamespace, 0, 240, 3),
		"deployment replicas should be 0 after 3 minutes")

	// Make a request and verify placeholder is served
	testPlaceholderResponse(t)

	// Test that placeholder is served immediately on cold start
	testImmediatePlaceholderResponse(t)
}

func testPlaceholderResponse(t *testing.T) {
	t.Log("Testing placeholder page response")

	var interceptorIP string
	if UseIngressHost {
		interceptorIP = ExternalIP
	} else {
		interceptorService := GetKubernetesServiceEndpoint(
			t,
			KubeClient,
			"keda-http-add-on-interceptor-proxy",
			"keda",
		)
		interceptorIP = interceptorService.IP
	}

	url := fmt.Sprintf("http://%s", interceptorIP)
	req, err := http.NewRequest("GET", url, nil)
	assert.NoError(t, err)
	req.Host = testName

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check that we got a 503 status (default for placeholder)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Read response body
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	bodyStr := string(body[:n])

	// Verify placeholder content
	assert.True(t, strings.Contains(bodyStr, testServiceName+" is starting up..."),
		"response should contain placeholder message")
	assert.True(t, strings.Contains(bodyStr, "refresh"),
		"response should contain refresh meta tag")

	// Check custom headers
	assert.Equal(t, "true", resp.Header.Get("X-KEDA-HTTP-Placeholder-Served"),
		"placeholder served header should be present")
	assert.Equal(t, "warming-up", resp.Header.Get("X-Service-Status"),
		"custom header should be present")
}

func testImmediatePlaceholderResponse(t *testing.T) {
	t.Log("Testing immediate placeholder page response on cold start")

	var interceptorIP string
	if UseIngressHost {
		interceptorIP = ExternalIP
	} else {
		interceptorService := GetKubernetesServiceEndpoint(
			t,
			KubeClient,
			"keda-http-add-on-interceptor-proxy",
			"keda",
		)
		interceptorIP = interceptorService.IP
	}

	url := fmt.Sprintf("http://%s", interceptorIP)
	req, err := http.NewRequest("GET", url, nil)
	assert.NoError(t, err)
	req.Host = testName

	client := &http.Client{
		Timeout: 2 * time.Second, // Short timeout to ensure immediate response
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Measure response time
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check that response was immediate (less than 500ms)
	assert.Less(t, duration.Milliseconds(), int64(500),
		"placeholder should be served immediately, but took %v", duration)

	// Check that we got a 503 status
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Check placeholder header
	assert.Equal(t, "true", resp.Header.Get("X-KEDA-HTTP-Placeholder-Served"),
		"placeholder served header should be present")
}
