//go:build stress
// +build stress

package stress

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	rapidTestName = "rapid-scaling-stress-test"
)

var (
	rapidTestNamespace            = fmt.Sprintf("%s-ns", rapidTestName)
	rapidDeploymentName           = fmt.Sprintf("%s-deployment", rapidTestName)
	rapidServiceName              = fmt.Sprintf("%s-service", rapidTestName)
	rapidHTTPScaledObjectName     = fmt.Sprintf("%s-http-so", rapidTestName)
	rapidHost                     = rapidTestName
	rapidMinReplicaCount          = 0 // Scale to zero when no load
	rapidMaxReplicaCount          = 8
	rapidBaseLoadExpectedReplicas = 1 // Expected replicas with base-load running (3 req/s < targetValue 5)

	// Thresholds for "rapid" scaling validation
	rapidScaleUpThreshold   = 30 * time.Second  // Max time to scale up to target replicas
	rapidScaleDownThreshold = 360 * time.Second // Max time to scale down after load stops
)

type rapidTemplateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
	JobName              string
	Duration             string
	Concurrency          string
}

const (
	rapidServiceTemplate = `
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

	rapidWorkloadTemplate = `
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

	// Base load: very low concurrency with rate limit, runs for entire test duration
	// Uses -q (queries per second) to limit request rate to ~3 req/s total
	// This should keep ~1 replica active (targetValue=5)
	rapidBaseLoadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: base-load-generator
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
          - "-z"
          - "600s"
          - "-c"
          - "1"
          - "-q"
          - "3"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 900
  backoffLimit: 1
`

	// Burst load: high concurrency, duration-based
	rapidBurstLoadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.JobName}}
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
          - "-z"
          - "{{.Duration}}"
          - "-c"
          - "{{.Concurrency}}"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 300
  backoffLimit: 1
`

	rapidHTTPScaledObjectTemplate = `
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
      targetValue: 5
      window: 30s
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

func TestRapidScalingStress(t *testing.T) {
	// setup
	t.Log("--- setting up rapid scaling stress test ---")
	kc := GetKubernetesClient(t)
	data, templates := getRapidTemplateData()
	CreateKubernetesResources(t, kc, rapidTestNamespace, data, templates)

	// OPTIONAL: Uncomment below to use different HPA stabilization window (30s instead of 5min)
	// This patches the ScaledObject to speed up scale-down (not recommended for CI)
	// See https://github.com/kedacore/http-add-on/issues/1457 for native HTTPScaledObject support
	// NOTE: If you enable the patch below, the current rapidScaleDownThreshold should be adjusted to 60s.
	/*
		t.Log("--- patching ScaledObject for faster scale-down ---")
		patchCmd := fmt.Sprintf(
			`kubectl patch scaledobject %s -n %s --type=merge -p '{"spec":{"advanced":{"horizontalPodAutoscalerConfig":{"behavior":{"scaleDown":{"stabilizationWindowSeconds":30}}}}}}'`,
			rapidHTTPScaledObjectName, rapidTestNamespace)
		_, err := ExecuteCommand(patchCmd)
		require.NoError(t, err, "failed to patch ScaledObject stabilization window")
	*/

	// Wait for initial state (0 replicas before any load)
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, 0, 6, 10),
		"replica count should be 0 after 1 minute")

	// Start base load (runs for entire test)
	t.Log("--- starting base load generator ---")
	KubectlApplyWithTemplate(t, data, "rapidBaseLoadJobTemplate", rapidBaseLoadJobTemplate)

	// Wait for base load to trigger initial scaling
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidBaseLoadExpectedReplicas, 12, 5),
		"replica count should reach %d with base load", rapidBaseLoadExpectedReplicas)

	// Run multiple rapid scale up/down cycles with burst load
	testRapidScalingCycles(t, kc, data)

	// Stop base load
	t.Log("--- stopping base load generator ---")
	KubectlDeleteWithTemplate(t, data, "rapidBaseLoadJobTemplate", rapidBaseLoadJobTemplate)

	// Verify scale down to 0 after all load stops
	// HPA stabilization window is 5 minutes by default, so wait up to 6 minutes (72 * 5s)
	// If you enabled the ScaledObject patch above, you can use (18, 5) instead for faster testing
	t.Log("--- verifying final scale down to 0 ---")
	//require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, 0, 18, 5),
	//	"replica count should return to 0 after all load stops")
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, 0, 72, 5),
		"replica count should return to 0 after all load stops")

	// cleanup
	DeleteKubernetesResources(t, rapidTestNamespace, data, templates)
}

func testRapidScalingCycles(t *testing.T, kc *kubernetes.Clientset, data rapidTemplateData) {
	cycles := 3
	var scaleUpTimes []time.Duration
	var scaleDownTimes []time.Duration

	for i := 1; i <= cycles; i++ {
		t.Logf("--- testing rapid scaling cycle %d of %d ---", i, cycles)

		// Generate unique job name for this cycle
		data.JobName = fmt.Sprintf("burst-load-%d", i)

		// Scale up phase - start burst load and measure time
		t.Logf("--- cycle %d: starting burst load ---", i)
		scaleUpStart := time.Now()
		KubectlApplyWithTemplate(t, data, "rapidBurstLoadJobTemplate", rapidBurstLoadJobTemplate)

		// Wait for scale up (burst + base load should trigger more replicas)
		require.True(t, WaitForDeploymentReplicaReadyMinCount(t, kc, rapidDeploymentName, rapidTestNamespace, 4, 12, 5),
			"replica count should reach at least 4 in cycle %d", i)
		scaleUpDuration := time.Since(scaleUpStart)
		scaleUpTimes = append(scaleUpTimes, scaleUpDuration)
		t.Logf("--- cycle %d: scale-up completed in %v (threshold: %v) ---", i, scaleUpDuration.Round(time.Second), rapidScaleUpThreshold)
		require.LessOrEqual(t, scaleUpDuration, rapidScaleUpThreshold,
			"scale-up in cycle %d took %v, exceeds threshold %v", i, scaleUpDuration, rapidScaleUpThreshold)

		// Wait for burst load job to complete before deleting
		t.Logf("--- cycle %d: waiting for burst load to complete ---", i)
		WaitForJobSuccess(t, kc, data.JobName, rapidTestNamespace, 18, 5) // max 90s

		// Delete burst load (base load continues) and measure scale-down time
		t.Logf("--- cycle %d: stopping burst load ---", i)
		scaleDownStart := time.Now()
		KubectlDeleteWithTemplate(t, data, "rapidBurstLoadJobTemplate", rapidBurstLoadJobTemplate)

		// Scale down phase - should scale down to ~1 replica due to base load (3 req/s)
		// HPA stabilization window is 5 minutes by default, so wait up to 6 minutes (72 * 5s)
		// If you enabled the ScaledObject patch above, you can use (18, 5) instead for faster testing
		t.Logf("--- cycle %d: scale down phase (base load still running) ---", i)
		require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidBaseLoadExpectedReplicas, 72, 5),
			"replica count should return to %d in cycle %d", rapidBaseLoadExpectedReplicas, i)
		scaleDownDuration := time.Since(scaleDownStart)
		scaleDownTimes = append(scaleDownTimes, scaleDownDuration)
		t.Logf("--- cycle %d: scale-down completed in %v (threshold: %v) ---", i, scaleDownDuration.Round(time.Second), rapidScaleDownThreshold)
		require.LessOrEqual(t, scaleDownDuration, rapidScaleDownThreshold,
			"scale-down in cycle %d took %v, exceeds threshold %v", i, scaleDownDuration, rapidScaleDownThreshold)

		// Brief pause between cycles
		if i < cycles {
			t.Logf("--- brief pause before next cycle ---")
			time.Sleep(10 * time.Second)
		}
	}

	// Summary of scaling performance
	t.Log("--- rapid scaling cycles completed successfully ---")
	t.Logf("--- scaling performance summary ---")
	for i, dur := range scaleUpTimes {
		t.Logf("  Cycle %d: Scale-Up: %v, Scale-Down: %v", i+1, dur.Round(time.Second), scaleDownTimes[i].Round(time.Second))
	}
}

func getRapidTemplateData() (rapidTemplateData, []Template) {
	return rapidTemplateData{
			TestNamespace:        rapidTestNamespace,
			DeploymentName:       rapidDeploymentName,
			ServiceName:          rapidServiceName,
			HTTPScaledObjectName: rapidHTTPScaledObjectName,
			Host:                 rapidHost,
			MinReplicas:          rapidMinReplicaCount,
			MaxReplicas:          rapidMaxReplicaCount,
			JobName:              "burst-load-1",
			Duration:             "60s",
			Concurrency:          "20",
		}, []Template{
			{Name: "rapidWorkloadTemplate", Config: rapidWorkloadTemplate},
			{Name: "rapidServiceNameTemplate", Config: rapidServiceTemplate},
			{Name: "rapidHTTPScaledObjectTemplate", Config: rapidHTTPScaledObjectTemplate},
		}
}
