//go:build e2e
// +build e2e

package utils

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

func TestVerifyCommands(t *testing.T) {
	commands := []string{"kubectl"}
	for _, cmd := range commands {
		_, err := exec.LookPath(cmd)
		require.NoErrorf(t, err, "%s is required for setup - %s", cmd, err)
	}
}

func TestKubernetesConnection(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
}

func TestKubernetesVersion(t *testing.T) {
	out, err := ExecuteCommand("kubectl version")
	require.NoErrorf(t, err, "error getting kubernetes version - %s", err)

	t.Logf("kubernetes version: %s", string(out))
}

func TestSetupHelm(t *testing.T) {
	_, err := exec.LookPath("helm")
	if err == nil {
		t.Skip("helm is already installed. skipping setup.")
	}

	_, err = ExecuteCommand("curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3")
	require.NoErrorf(t, err, "cannot download helm installation shell script - %s", err)

	_, err = ExecuteCommand("chmod 700 get_helm.sh")
	require.NoErrorf(t, err, "cannot change permissions for helm installation script - %s", err)

	_, err = ExecuteCommand("./get_helm.sh")
	require.NoErrorf(t, err, "cannot download helm - %s", err)

	_, err = ExecuteCommand("helm version")
	require.NoErrorf(t, err, "cannot get helm version - %s", err)
}

func TestCreateKEDANamespace(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, KEDANamespace)
}

func TestSetupArgoRollouts(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, ArgoRolloutsNamespace)
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add argo https://argoproj.github.io/argo-helm")
	require.NoErrorf(t, err, "cannot add argo helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update argo")
	require.NoErrorf(t, err, "cannot update argo helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install %s argo/argo-rollouts --namespace %s --wait",
		ArgoRolloutsName,
		ArgoRolloutsNamespace))
	require.NoErrorf(t, err, "cannot install argo-rollouts - %s", err)
}

func TestSetupIngress(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, IngressNamespace)
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx")
	require.NoErrorf(t, err, "cannot add ingress-nginx helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update ingress-nginx")
	require.NoErrorf(t, err, "cannot update ingress-nginx helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install %s ingress-nginx/ingress-nginx --set fullnameOverride=%s --set controller.service.type=ClusterIP --namespace %s --wait",
		IngressReleaseName, IngressReleaseName, IngressNamespace))
	require.NoErrorf(t, err, "cannot install ingress - %s", err)
}

func TestSetupKEDA(t *testing.T) {
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add kedacore https://kedacore.github.io/charts")
	require.NoErrorf(t, err, "cannot add kedacore helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update kedacore")
	require.NoErrorf(t, err, "cannot update kedacore helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install keda kedacore/keda --namespace %s --wait --set extraArgs.keda.kube-api-qps=200 --set extraArgs.keda.kube-api-burst=300",
		KEDANamespace))
	require.NoErrorf(t, err, "cannot install KEDA - %s", err)
}

func TestDeployKEDAHttpAddOn(t *testing.T) {
	out, err := ExecuteCommandWithDir("make deploy", "../..")
	require.NoErrorf(t, err, "error deploying KEDA Http Add-on - %s", err)

	t.Log(string(out))
	t.Log("KEDA Http Add-on deployed successfully using 'make deploy' command")
}
