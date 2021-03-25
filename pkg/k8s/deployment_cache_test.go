package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	appsfake "k8s.io/client-go/kubernetes/typed/apps/v1/fake"
)

func TestK8DeploymentCacheGet(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	const ns = "testns"
	const name = "testdepl"
	expectedDepl := NewDeployment(
		ns,
		name,
		"testimg",
		nil,
		nil,
		make(map[string]string),
	)
	fakeClientset := k8sfake.NewSimpleClientset(expectedDepl)
	fakeApps := appsfake.FakeAppsV1{
		Fake: &fakeClientset.Fake,
	}

	cache, err := NewK8sDeploymentCache(ctx, fakeApps.Deployments(ns))
	r.NoError(err)

	depl, err := cache.Get(name)
	r.NoError(err)
	r.Equal(name, depl.ObjectMeta.Name)

	none, err := cache.Get(name + "noexist")
	r.NotNil(err)
	r.Nil(none)
}
