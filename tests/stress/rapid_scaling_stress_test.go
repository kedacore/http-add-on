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
	rapidMinReplicaCount          = int32(0) // Scale to zero when no load
	rapidMaxReplicaCount          = int32(8)
	rapidBaseLoadExpectedReplicas = 1 // Expected replicas with base-load running (3 req/s < targetValue 5)

	// Thresholds for "rapid" scaling validation
	rapidScaleUpThreshold   = 30 * time.Second  // Max time to scale up to target replicas
	rapidScaleDownThreshold = 360 * time.Second // Max time to scale down after load stops
)

// Resource specifications using typed structs
var (
	rapidServiceSpec = ServiceSpec{
		Name:       rapidServiceName,
		Namespace:  rapidTestNamespace,
		App:        rapidDeploymentName,
		Port:       8080,
		TargetPort: "http",
	}

	rapidDeploymentSpec = DeploymentSpec{
		Name:          rapidDeploymentName,
		Namespace:     rapidTestNamespace,
		App:           rapidDeploymentName,
		Replicas:      0,
		Image:         "registry.k8s.io/e2e-test-images/agnhost:2.45",
		Args:          []string{"netexec"},
		ContainerPort: 8080,
		ReadinessPath: "/",
		ReadinessPort: "http",
	}

	rapidHTTPScaledObjectSpec = HTTPScaledObjectSpec{
		Name:            rapidHTTPScaledObjectName,
		Namespace:       rapidTestNamespace,
		Hosts:           []string{rapidHost},
		DeploymentName:  rapidDeploymentName,
		ServiceName:     rapidServiceName,
		Port:            8080,
		MinReplicas:     rapidMinReplicaCount,
		MaxReplicas:     rapidMaxReplicaCount,
		ScaledownPeriod: 10,
		TargetValue:     5,
		RateWindow:      30 * time.Second,
		RateGranularity: time.Second,
	}

	// Base load: very low concurrency with rate limit, runs for entire test duration
	// Uses -q (queries per second) to limit request rate to ~3 req/s total
	// This should keep ~1 replica active (targetValue=5)
	rapidBaseLoadJobSpec = OhaLoadJobSpec{
		Name:                       "base-load-generator",
		Namespace:                  rapidTestNamespace,
		Host:                       rapidHost,
		Duration:                   "600s",
		Concurrency:                1,
		RateLimit:                  3,
		TerminationGracePeriodSecs: 5,
		ActiveDeadlineSeconds:      900,
		BackoffLimit:               1,
	}
)

func TestRapidScalingStress(t *testing.T) {
	// setup
	t.Log("--- setting up rapid scaling stress test ---")
	kc := GetKubernetesClient(t)

	// Create namespace
	CreateNamespace(t, kc, rapidTestNamespace)

	// Apply resources using typed structs
	require.NoError(t, KubectlApplyYAML(t, "service", BuildServiceYAML(rapidServiceSpec)))
	require.NoError(t, KubectlApplyYAML(t, "deployment", BuildDeploymentYAML(rapidDeploymentSpec)))
	require.NoError(t, KubectlApplyYAML(t, "httpscaledobject", BuildHTTPScaledObjectYAML(rapidHTTPScaledObjectSpec)))

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
	require.NoError(t, KubectlApplyYAML(t, "base-load", BuildOhaLoadJobYAML(rapidBaseLoadJobSpec)))

	// Wait for base load to trigger initial scaling
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidBaseLoadExpectedReplicas, 12, 5),
		"replica count should reach %d with base load", rapidBaseLoadExpectedReplicas)

	// Run multiple rapid scale up/down cycles with burst load
	testRapidScalingCycles(t, kc)

	// Stop base load
	t.Log("--- stopping base load generator ---")
	_ = KubectlDeleteYAML(t, "base-load", BuildOhaLoadJobYAML(rapidBaseLoadJobSpec))

	// Verify scale down to 0 after all load stops
	// HPA stabilization window is 5 minutes by default, so wait up to 6 minutes (72 * 5s)
	// If you enabled the ScaledObject patch above, you can use (18, 5) instead for faster testing
	t.Log("--- verifying final scale down to 0 ---")
	//require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, 0, 18, 5),
	//	"replica count should return to 0 after all load stops")
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, 0, 72, 5),
		"replica count should return to 0 after all load stops")

	// cleanup
	DeleteNamespace(t, rapidTestNamespace)
}

func testRapidScalingCycles(t *testing.T, kc *kubernetes.Clientset) {
	cycles := 3
	var scaleUpTimes []time.Duration
	var scaleDownTimes []time.Duration

	for i := 1; i <= cycles; i++ {
		t.Logf("--- testing rapid scaling cycle %d of %d ---", i, cycles)

		// Create burst load job spec for this cycle
		burstJobSpec := OhaLoadJobSpec{
			Name:                       fmt.Sprintf("burst-load-%d", i),
			Namespace:                  rapidTestNamespace,
			Host:                       rapidHost,
			Duration:                   "60s",
			Concurrency:                20,
			TerminationGracePeriodSecs: 5,
			ActiveDeadlineSeconds:      300,
			BackoffLimit:               1,
		}

		// Scale up phase - start burst load and measure time
		t.Logf("--- cycle %d: starting burst load ---", i)
		scaleUpStart := time.Now()
		require.NoError(t, KubectlApplyYAML(t, burstJobSpec.Name, BuildOhaLoadJobYAML(burstJobSpec)))

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
		WaitForJobSuccess(t, kc, burstJobSpec.Name, rapidTestNamespace, 18, 5) // max 90s

		// Delete burst load (base load continues) and measure scale-down time
		t.Logf("--- cycle %d: stopping burst load ---", i)
		scaleDownStart := time.Now()
		_ = KubectlDeleteYAML(t, burstJobSpec.Name, BuildOhaLoadJobYAML(burstJobSpec))

		// Scale down phase - should scale down to ~1 replica due to base load (3 req/s)
		// HPA stabilization window is 5 minutes by default, so wait up to 6 minutes (84 * 5s)
		// If you enabled the ScaledObject patch above, you can use (18, 5) instead for faster testing
		t.Logf("--- cycle %d: scale down phase (base load still running) ---", i)
		require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidBaseLoadExpectedReplicas, 84, 5),
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
