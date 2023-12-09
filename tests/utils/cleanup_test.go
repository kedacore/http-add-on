//go:build e2e
// +build e2e

package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

func TestRemoveKEDAHttpAddOn(t *testing.T) {
	out, err := ExecuteCommandWithDir("make undeploy", "../..")
	require.NoErrorf(t, err, "error removing KEDA Http Add-on - %s", err)

	t.Log(string(out))
	t.Log("KEDA Http Add-on removed successfully using 'make undeploy' command")
}

func TestRemoveKEDA(t *testing.T) {
	_, err := ExecuteCommand(fmt.Sprintf("helm uninstall keda --namespace %s", KEDANamespace))
	require.NoErrorf(t, err, "cannot uninstall KEDA - %s", err)
}

func TestRemoveIngress(t *testing.T) {
	_, err := ExecuteCommand(fmt.Sprintf("helm uninstall %s --namespace %s", IngressReleaseName, IngressNamespace))
	require.NoErrorf(t, err, "cannot uninstall ingress - %s", err)
	DeleteNamespace(t, IngressNamespace)
}

func TestRemoveArgoRollouts(t *testing.T) {
	_, err := ExecuteCommand(fmt.Sprintf("helm uninstall %s --namespace %s", ArgoRolloutsName, ArgoRolloutsNamespace))
	require.NoErrorf(t, err, "cannot uninstall ingress - %s", err)
	DeleteNamespace(t, ArgoRolloutsNamespace)
}

func TestRemoveKEDANamespace(t *testing.T) {
	DeleteNamespace(t, KEDANamespace)
}
