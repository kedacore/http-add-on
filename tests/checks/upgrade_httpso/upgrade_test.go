//go:build e2e

// Package upgrade_test exercises a minimal Helm-chart upgrade path:
//   1. Install a previous release of the http-add-on chart.
//   2. Create a sample workload + HTTPScaledObject and verify it scales.
//   3. `helm upgrade` to the current chart version.
//   4. Verify the HTTPScaledObject still reconciles and the workload still scales.
//
// Skipped unless both HTTPADDON_UPGRADE_FROM_VERSION and HTTPADDON_UPGRADE_TO_VERSION
// are set (e.g. "0.10.0" and "0.11.0"). Intended to run as a dedicated CI job so the
// main e2e suite — which expects a single pre-installed add-on — is unaffected.
package upgrade_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "upgrade-httpso-test"
	// releaseName and chart ref match the chart documented at
	// https://github.com/kedacore/charts and referenced by tests/utils/setup_test.go.
	releaseName = "http-add-on"
	chartRef    = "kedacore/http-add-on"

	fromVersionEnv = "HTTPADDON_UPGRADE_FROM_VERSION"
	toVersionEnv   = "HTTPADDON_UPGRADE_TO_VERSION"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 1
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
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
        command: ["curl", "-H", "Host: {{.Host}}", "keda-add-ons-http-interceptor-proxy.keda:8080"]
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

func TestUpgrade(t *testing.T) {
	fromVersion := os.Getenv(fromVersionEnv)
	toVersion := os.Getenv(toVersionEnv)
	if fromVersion == "" || toVersion == "" {
		t.Skipf("skipping upgrade scenario: set %s and %s to run", fromVersionEnv, toVersionEnv)
	}

	kc := GetKubernetesClient(t)

	t.Logf("--- installing http-add-on %s (baseline) ---", fromVersion)
	installOrUpgradeAddon(t, fromVersion)
	waitForAddonReady(t, kc)

	data, templates := getTemplateData()

	t.Log("--- creating workload on baseline ---")
	CreateKubernetesResources(t, kc, testNamespace, data, templates)
	t.Cleanup(func() {
		DeleteKubernetesResources(t, testNamespace, data, templates)
	})

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after setup", minReplicaCount)

	t.Log("--- scale out (baseline) ---")
	testScaleOut(t, kc, data)

	t.Logf("--- upgrading http-add-on to %s ---", toVersion)
	installOrUpgradeAddon(t, toVersion)
	waitForAddonReady(t, kc)

	t.Log("--- HTTPScaledObject survives upgrade ---")
	// Give the new operator a moment to reconcile pre-existing HTTPScaledObject resources.
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"expected HTTPScaledObject to still route traffic and keep the deployment at max replicas post-upgrade")

	t.Log("--- scale in (post-upgrade) ---")
	testScaleIn(t, kc, data)

	t.Log("--- scale out again (post-upgrade) ---")
	testScaleOut(t, kc, data)
}

func installOrUpgradeAddon(t *testing.T, version string) {
	t.Helper()
	_, err := ExecuteCommand("helm repo add kedacore https://kedacore.github.io/charts")
	require.NoErrorf(t, err, "cannot add kedacore helm repo - %s", err)
	_, err = ExecuteCommand("helm repo update kedacore")
	require.NoErrorf(t, err, "cannot update kedacore helm repo - %s", err)
	_, err = ExecuteCommand(fmt.Sprintf(
		"helm upgrade --install %s %s --version %s --namespace %s --wait",
		releaseName, chartRef, version, KEDANamespace,
	))
	require.NoErrorf(t, err, "cannot install/upgrade %s to version %s - %s", releaseName, version, err)
}

func waitForAddonReady(t *testing.T, kc *kubernetes.Clientset) {
	t.Helper()
	// Same deployments TestSetupKEDA waits for — names are stable across the releases we support upgrading between.
	for _, name := range []string{"keda-add-ons-http-operator", "keda-add-ons-http-interceptor", "keda-add-ons-http-external-scaler"} {
		assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, name, KEDANamespace, 1, 30, 6),
			"%s not ready after upgrade", name)
	}
}

func testScaleOut(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Helper()
	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should be %d after load", maxReplicaCount)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Helper()
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after load stops", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			DeploymentName:       deploymentName,
			ServiceName:          serviceName,
			HTTPScaledObjectName: httpScaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "deploymentTemplate", Config: deploymentTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}
