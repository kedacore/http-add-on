package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
		"Always",
	)
	fakeClientset := k8sfake.NewSimpleClientset(expectedDepl)
	fakeApps := fakeClientset.AppsV1()

	cache, err := NewK8sDeploymentCache(ctx, fakeApps.Deployments(ns))
	r.NoError(err)

	depl, err := cache.Get(name)
	r.NoError(err)
	r.Equal(name, depl.ObjectMeta.Name)

	none, err := cache.Get(name + "noexist")
	r.NotNil(err)
	r.Nil(none)
}

func TestK8sDeploymentCacheWatch(t *testing.T) {
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
		"Always",
	)
	fakeClientset := k8sfake.NewSimpleClientset()
	fakeDeployments := fakeClientset.AppsV1().Deployments(ns)

	cache, err := NewK8sDeploymentCache(ctx, fakeDeployments)
	r.NoError(err)

	watcher := cache.Watch(name)
	defer watcher.Stop()

	createSentCh := make(chan struct{})
	createErrCh := make(chan error)
	go func() {
		time.Sleep(200 * time.Millisecond)
		_, err := fakeDeployments.Create(
			ctx,
			expectedDepl,
			metav1.CreateOptions{},
		)
		if err != nil {
			createErrCh <- err
		} else {
			close(createSentCh)
		}
	}()

	// first make sure that the send happened, and there was no error
	select {
	case <-createSentCh:
	case err := <-createErrCh:
		r.NoError(err, "error creating the new deployment to trigger the event")
	case <-time.After(400 * time.Millisecond):
		r.Fail("the create operation didn't happen after 400 ms")
	}

	// then make sure that the deployment was actually received
	select {
	case obj := <-watcher.ResultChan():
		depl, ok := obj.Object.(*appsv1.Deployment)
		r.True(ok, "expected a deployment but got a %#V", obj)
		r.Equal(ns, depl.ObjectMeta.Namespace)
		r.Equal(name, depl.ObjectMeta.Name)
	case <-time.After(500 * time.Millisecond):
		r.Fail("didn't get a watch event after 500 ms")
	}
}

func gvrForDeployment(depl *appsv1.Deployment) schema.GroupVersionResource {
	gvk := depl.GroupVersionKind()
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "Deployment",
	}
}
