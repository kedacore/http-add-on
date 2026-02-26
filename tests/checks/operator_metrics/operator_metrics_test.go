//go:build e2e

package operator_metrics_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "operator-metrics-test"
)

var (
	testNamespace          = fmt.Sprintf("%s-ns", testName)
	clientName             = fmt.Sprintf("%s-client", testName)
	kedaOperatorMetricsURL = "http://keda-add-ons-http-operator-metrics.keda:8080/metrics"
)

type templateData struct {
	TestNamespace string
	ClientName    string
}

const (
	clientTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}
spec:
  serviceAccountName: {{.ClientName}}
  containers:
  - name: {{.ClientName}}
    image: curlimages/curl
    command:
      - sh
      - -c
      - "exec tail -f /dev/null"`

	serviceAccountTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}`

	clusterRoleBindingTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{.ClientName}}-metrics-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{.ClientName}}-metrics-reader
subjects:
- kind: ServiceAccount
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}`

	clusterRoleTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{.ClientName}}-metrics-reader
rules:
- nonResourceURLs:
  - "/metrics"
  verbs:
  - get`
)

func TestOperatorMetrics(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	// Wait for client pod to be ready
	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, testNamespace, 6, 10),
		"client pod should be running")

	t.Log("--- testing operator metrics endpoint ---")

	// Test 1: HTTPS endpoint should be accessible (will fail cert validation but should return metrics)
	t.Log("Test 1: Verify HTTP endpoint is available")
	testHTTPEndpoint(t)

	// Test 2: Verify metrics are returned
	t.Log("Test 2: Verify metrics content")
	testMetricsContent(t)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testHTTPEndpoint(t *testing.T) {
	cmd := fmt.Sprintf("curl --max-time 10 %s", kedaOperatorMetricsURL)
	out, errOut, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, cmd)

	// The endpoint should return something (even if authentication fails, it should respond)
	assert.True(t, err == nil || strings.Contains(errOut, "Forbidden") || strings.Contains(out, "Forbidden"),
		"HTTP endpoint should respond (either with metrics or authentication error)")
}

func testMetricsContent(t *testing.T) {
	// Access metrics from the client pod using the service endpoint
	// The client pod uses a ServiceAccount with the operator-metrics-reader ClusterRole
	// This allows it to access the metrics endpoint with proper RBAC permissions

	// Get the ServiceAccount token to authenticate
	cmd := fmt.Sprintf("curl -H \"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" --max-time 10 %s", kedaOperatorMetricsURL)
	out, errOut, err := ExecCommandOnSpecificPod(t, clientName, testNamespace, cmd)
	if err != nil {
		t.Logf("Metrics content test - Output: %s, Error output: %s, Error: %v", out, errOut, err)
	}

	// Verify that metrics are returned in Prometheus format
	require.NoError(t, err, "should be able to access metrics from client pod with RBAC permissions. Output: %s, Error: %s", out, errOut)
	assert.True(t, strings.Contains(out, "# HELP") || strings.Contains(out, "# TYPE"),
		"metrics should contain Prometheus format. Output: %s", out)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace: testNamespace,
			ClientName:    clientName,
		}, []Template{
			{Name: "serviceAccountTemplate", Config: serviceAccountTemplate},
			{Name: "clusterRoleBindingTemplate", Config: clusterRoleBindingTemplate},
			{Name: "clusterRoleTemplate", Config: clusterRoleTemplate},
			{Name: "clientTemplate", Config: clientTemplate},
		}
}
