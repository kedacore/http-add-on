//go:build e2e
// +build e2e

package scaling_set_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	scaling_set_helper "github.com/kedacore/http-add-on/tests/checks/scaling_set/helper"
	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName     = "cluster-scaling-set-test"
	clusterScope = true
)

func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := scaling_set_helper.GetTemplateData(testName, clusterScope)
	CreateKubernetesResources(t, kc, data.TestNamespace, data, templates)

	scaling_set_helper.WaitForScalingSetComponents(t, kc, data)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, data.DeploymentName, data.TestNamespace, data.MinReplicas, 6, 10),
		"replica count should be %d after 1 minutes", data.MinReplicas)

	testScaleOut(t, kc, data)
	testScaleIn(t, kc, data)

	// cleanup
	DeleteKubernetesResources(t, data.TestNamespace, data, templates)
}

func testScaleOut(t *testing.T, kc *kubernetes.Clientset, data scaling_set_helper.TemplateData) {
	t.Log("--- testing scale out ---")

	loadTemplate := scaling_set_helper.GetLoadJobTemplate()
	KubectlApplyWithTemplate(t, data, loadTemplate.Name, loadTemplate.Config)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, data.DeploymentName, data.TestNamespace, data.MaxReplicas, 6, 20),
		"replica count should be %d after 2 minutes", data.MaxReplicas)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset, data scaling_set_helper.TemplateData) {
	t.Log("--- testing scale out ---")

	loadTemplate := scaling_set_helper.GetLoadJobTemplate()
	KubectlDeleteWithTemplate(t, data, loadTemplate.Name, loadTemplate.Config)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, data.DeploymentName, data.TestNamespace, data.MinReplicas, 18, 10),
		"replica count should be %d after 3 minutes", data.MinReplicas)
}
