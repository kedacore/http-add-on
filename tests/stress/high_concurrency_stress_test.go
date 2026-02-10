//go:build stress
// +build stress

package stress

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "high-concurrency-stress-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	deploymentName       = fmt.Sprintf("%s-deployment", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 10

	// Thresholds for high concurrency stress test validation
	scaleUpThreshold   = 80 * time.Second  // Max time to scale up to max replicas under high load
	scaleDownThreshold = 120 * time.Second // Max time to scale down after load stops
)

type templateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
	Requests             string
	Concurrency          string
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

	workloadTemplate = `
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
  name: load-generator
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: oha
        image: ghcr.io/hatoo/oha:1.13
        imagePullPolicy: Always
        args:
          - "--no-tui"
          - "-n"
          - "{{.Requests}}"
          - "-c"
          - "{{.Concurrency}}"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 1200
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
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 10
      window: 1m
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

func TestHighConcurrencyStress(t *testing.T) {
	// setup
	t.Log("--- setting up high concurrency stress test ---")
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	// OPTIONAL: Uncomment below to use different HPA stabilization window (30s instead of 5min)
	// This patches the ScaledObject to speed up scale-down (not recommended for CI)
	// See https://github.com/kedacore/http-add-on/issues/1457 for native HTTPScaledObject support
	// NOTE: If you enable the patch below, the current scaleDownThreshold should be adjusted to 60s.
	/*
		t.Log("--- patching ScaledObject for faster scale-down ---")
		patchCmd := fmt.Sprintf(
			`kubectl patch scaledobject %s -n %s --type=merge -p '{"spec":{"advanced":{"horizontalPodAutoscalerConfig":{"behavior":{"scaleDown":{"stabilizationWindowSeconds":30}}}}}}'`,
			httpScaledObjectName, testNamespace)
		_, err := ExecuteCommand(patchCmd)
		require.NoError(t, err, "failed to patch ScaledObject stabilization window")
	*/

	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	testHighConcurrencyLoad(t, kc, data)
	testScaleIn(t, kc)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testHighConcurrencyLoad(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing high concurrency load (500,000 requests with 50 concurrent connections) ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	// Wait for scaling up to max replicas and measure duration
	scaleUpStart := time.Now()
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 36, 10),
		"replica count should be %d after 6 minutes under high load", maxReplicaCount)
	scaleUpDuration := time.Since(scaleUpStart)

	t.Logf("--- scale-up completed in %v (threshold: %v) ---", scaleUpDuration.Round(time.Second), scaleUpThreshold)
	require.LessOrEqual(t, scaleUpDuration, scaleUpThreshold,
		"scale-up took %v, exceeds threshold %v", scaleUpDuration, scaleUpThreshold)

	// Verify the system remains stable at max replicas
	t.Log("--- verifying system stability at max replicas ---")
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, maxReplicaCount, 6, 10),
		"replica count should remain at %d", maxReplicaCount)

	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing scale in after stress test ---")

	scaleDownStart := time.Now()
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, minReplicaCount, 24, 10),
		"replica count should be %d after 4 minutes", minReplicaCount)
	scaleDownDuration := time.Since(scaleDownStart)

	t.Logf("--- scale-down completed in %v (threshold: %v) ---", scaleDownDuration.Round(time.Second), scaleDownThreshold)
	require.LessOrEqual(t, scaleDownDuration, scaleDownThreshold,
		"scale-down took %v, exceeds threshold %v", scaleDownDuration, scaleDownThreshold)
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
			Requests:             "500000",
			Concurrency:          "50",
		}, []Template{
			{Name: "workloadTemplate", Config: workloadTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}
