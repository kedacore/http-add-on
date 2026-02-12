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
	minReplicaCount      = int32(0)
	maxReplicaCount      = int32(10)

	// Thresholds for high concurrency stress test validation
	scaleUpThreshold   = 80 * time.Second  // Max time to scale up to max replicas under high load
	scaleDownThreshold = 120 * time.Second // Max time to scale down after load stops
)

// Resource specifications using typed structs
var (
	serviceSpec = ServiceSpec{
		Name:       serviceName,
		Namespace:  testNamespace,
		App:        deploymentName,
		Port:       8080,
		TargetPort: "http",
	}

	deploymentSpec = DeploymentSpec{
		Name:          deploymentName,
		Namespace:     testNamespace,
		App:           deploymentName,
		Replicas:      0,
		Image:         "registry.k8s.io/e2e-test-images/agnhost:2.45",
		Args:          []string{"netexec"},
		ContainerPort: 8080,
		ReadinessPath: "/",
		ReadinessPort: "http",
	}

	httpScaledObjectSpec = HTTPScaledObjectSpec{
		Name:            httpScaledObjectName,
		Namespace:       testNamespace,
		Hosts:           []string{host},
		DeploymentName:  deploymentName,
		ServiceName:     serviceName,
		Port:            8080,
		MinReplicas:     minReplicaCount,
		MaxReplicas:     maxReplicaCount,
		ScaledownPeriod: 10,
		TargetValue:     10,
		RateWindow:      time.Minute,
		RateGranularity: time.Second,
	}
)

func TestHighConcurrencyStress(t *testing.T) {
	// setup
	t.Log("--- setting up high concurrency stress test ---")
	kc := GetKubernetesClient(t)

	// Create namespace
	CreateNamespace(t, kc, testNamespace)

	// Apply resources using typed structs
	require.NoError(t, KubectlApplyYAML(t, "service", BuildServiceYAML(serviceSpec)))
	require.NoError(t, KubectlApplyYAML(t, "deployment", BuildDeploymentYAML(deploymentSpec)))
	require.NoError(t, KubectlApplyYAML(t, "httpscaledobject", BuildHTTPScaledObjectYAML(httpScaledObjectSpec)))

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

	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, int(minReplicaCount), 6, 10),
		"replica count should be %d after 1 minute", minReplicaCount)

	testHighConcurrencyLoad(t, kc)
	testScaleIn(t, kc)

	// cleanup
	DeleteNamespace(t, testNamespace)
}

func testHighConcurrencyLoad(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing high concurrency load (500,000 requests with 50 concurrent connections) ---")

	loadJobSpec := OhaLoadJobSpec{
		Name:                       "load-generator",
		Namespace:                  testNamespace,
		Host:                       host,
		Requests:                   500000,
		Concurrency:                50,
		TerminationGracePeriodSecs: 5,
		ActiveDeadlineSeconds:      1200,
		BackoffLimit:               5,
	}
	require.NoError(t, KubectlApplyYAML(t, "load-generator", BuildOhaLoadJobYAML(loadJobSpec)))

	// Wait for scaling up to max replicas and measure duration
	scaleUpStart := time.Now()
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, int(maxReplicaCount), 36, 10),
		"replica count should be %d after 6 minutes under high load", maxReplicaCount)
	scaleUpDuration := time.Since(scaleUpStart)

	t.Logf("--- scale-up completed in %v (threshold: %v) ---", scaleUpDuration.Round(time.Second), scaleUpThreshold)
	require.LessOrEqual(t, scaleUpDuration, scaleUpThreshold,
		"scale-up took %v, exceeds threshold %v", scaleUpDuration, scaleUpThreshold)

	// Verify the system remains stable at max replicas
	t.Log("--- verifying system stability at max replicas ---")
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, int(maxReplicaCount), 6, 10),
		"replica count should remain at %d", maxReplicaCount)

	_ = KubectlDeleteYAML(t, "load-generator", BuildOhaLoadJobYAML(loadJobSpec))
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing scale in after stress test ---")

	scaleDownStart := time.Now()
	require.True(t, WaitForDeploymentReplicaReadyCount(t, kc, deploymentName, testNamespace, int(minReplicaCount), 24, 10),
		"replica count should be %d after 4 minutes", minReplicaCount)
	scaleDownDuration := time.Since(scaleDownStart)

	t.Logf("--- scale-down completed in %v (threshold: %v) ---", scaleDownDuration.Round(time.Second), scaleDownThreshold)
	require.LessOrEqual(t, scaleDownDuration, scaleDownThreshold,
		"scale-down took %v, exceeds threshold %v", scaleDownDuration, scaleDownThreshold)
}
