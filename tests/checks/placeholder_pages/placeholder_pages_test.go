//go:build e2e
// +build e2e

package placeholder_pages

import (
	"fmt"
	"testing"

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
    deployment: {{.TestName}}
    service: {{.TestName}}
    port: 80
  replicas:
    min: 0
    max: 10
  placeholderConfig:
    enabled: true
    statusCode: 503
    refreshInterval: 5
    headers:
      X-Test-Header: "test-value"
`

func TestPlaceholderPages(t *testing.T) {
	// Setup
	data := templateData{
		TestNamespace: testNamespace,
		TestName:      testName,
	}

	KubectlApplyWithTemplate(t, data, "placeholder-test", testTemplate)
	defer func() {
		KubectlDeleteWithTemplate(t, data, "placeholder-test", testTemplate)
		DeleteNamespace(t, testNamespace)
	}()

	// Wait for deployment to scale to 0
	assert.True(t,
		WaitForDeploymentReplicaReadyCount(t, GetKubernetesClient(t), testName, testNamespace, 0, 60, 3),
		"deployment should scale to 0")

	// Make a request through a pod and check placeholder response
	curlCmd := fmt.Sprintf("curl -i -H 'Host: %s.test' http://keda-add-ons-http-interceptor-proxy.keda:8080/", testName)

	// Create and run a curl pod
	curlPod := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: curl-test
  namespace: %s
spec:
  containers:
  - name: curl
    image: curlimages/curl
    command: ["sh", "-c", "%s && sleep 5"]
  restartPolicy: Never
`, testNamespace, curlCmd)

	KubectlApplyWithTemplate(t, data, "curl-pod", curlPod)
	defer KubectlDeleteWithTemplate(t, data, "curl-pod", curlPod)

	// Wait and get logs
	assert.True(t,
		WaitForSuccessfulExecCommandOnSpecificPod(t, "curl-test", testNamespace, "echo done", 60, 3),
		"curl should complete")

	logs, _ := KubectlLogs(t, "curl-test", testNamespace, "")

	// Verify placeholder response
	assert.Contains(t, logs, "HTTP/1.1 503", "should return 503 status")
	assert.Contains(t, logs, "X-KEDA-HTTP-Placeholder-Served: true", "should have placeholder header")
	assert.Contains(t, logs, "X-Test-Header: test-value", "should have custom header")
	assert.Contains(t, logs, "is starting up", "should have placeholder message")
}
