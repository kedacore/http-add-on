package k8s

import (
	"context"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

type DeploymentCache interface {
	Get(name string) (*appsv1.Deployment, error)
	Watch(name string) watch.Interface
}

type K8sDeploymentCache struct {
	latestEvts  map[string]watch.Event
	rwm         *sync.RWMutex
	broadcaster *watch.Broadcaster
}

func NewK8sDeploymentCache(
	ctx context.Context,
	cl typedappsv1.DeploymentInterface,
) (*K8sDeploymentCache, error) {
	deployList, err := cl.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	latestEvts := map[string]watch.Event{}
	for _, depl := range deployList.Items {
		latestEvts[depl.ObjectMeta.Name] = watch.Event{
			Type:   watch.Added,
			Object: &depl,
		}
	}
	bcaster := watch.NewBroadcaster(5, watch.DropIfChannelFull)
	watcher, err := cl.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := &K8sDeploymentCache{
		latestEvts:  latestEvts,
		rwm:         new(sync.RWMutex),
		broadcaster: bcaster,
	}
	go func() {
		defer watcher.Stop()
		ch := watcher.ResultChan()
		for {
			// TODO: add a timeout
			evt := <-ch
			ret.broadcaster.Action(evt.Type, evt.Object)
			ret.rwm.Lock()
			depl := evt.Object.(*appsv1.Deployment)
			ret.latestEvts[depl.GetObjectMeta().GetName()] = evt
			ret.rwm.Unlock()
		}
	}()
	return ret, nil
}

func (k *K8sDeploymentCache) Get(name string) (*appsv1.Deployment, error) {
	k.rwm.RLock()
	defer k.rwm.RUnlock()
	evt, ok := k.latestEvts[name]
	if !ok {
		return nil, fmt.Errorf("No deployment %s found", name)
	}
	return evt.Object.(*appsv1.Deployment), nil
}

func (k *K8sDeploymentCache) Watch(name string) watch.Interface {
	watcher := k.broadcaster.Watch()
	return watch.Filter(watcher, func(evt watch.Event) (watch.Event, bool) {
		depl, ok := evt.Object.(*appsv1.Deployment)
		if !ok {
			return evt, false
		}
		if depl.ObjectMeta.Name != name {
			return evt, false
		}
		return evt, true
	})
}

// MemoryDeploymentCache is a purely in-memory DeploymentCache implementation. It's not
// concurrency-safe and intended to be used in tests only
type MemoryDeploymentCache struct {
	// Watchers holds watchers to be returned by calls to Watch. If Watch is called with a
	// name that has a key in this map, that function will panic. Otherwise, it will
	// return the corresponding value
	Watchers map[string]*watch.FakeWatcher

	// Deployments holds the deployments to be returned in calls to Get. If Get is called
	// with a name that exists as a key in this map, the corresponding value will be returned.
	// Otherwise, an error will be returned
	Deployments map[string]*appsv1.Deployment
}

// NewMemoryDeploymentCache creates a new MemoryDeploymentCache with the Deployments map set to
// initialDeployments, and the Watchers map initialized with a newly created and otherwise
// untouched FakeWatcher for each key in the initialDeployments map
func NewMemoryDeploymentCache(
	initialDeployments map[string]*appsv1.Deployment,
) *MemoryDeploymentCache {
	ret := &MemoryDeploymentCache{
		Watchers:    make(map[string]*watch.FakeWatcher),
		Deployments: make(map[string]*appsv1.Deployment),
	}
	ret.Deployments = initialDeployments
	for deployName := range initialDeployments {
		ret.Watchers[deployName] = watch.NewFake()
	}
	return ret
}

func (m *MemoryDeploymentCache) Get(name string) (*appsv1.Deployment, error) {
	val, ok := m.Deployments[name]
	if !ok {
		return nil, fmt.Errorf("Deployment %s not found", name)
	}
	return val, nil
}

func (m *MemoryDeploymentCache) Watch(name string) watch.Interface {
	val, ok := m.Watchers[name]
	if !ok {
		errString := fmt.Sprintf(
			"(github.com/kedacore/http-add-on/pkg/k8s).MemoryDeploymentCacher.Watch(%s) called, but that name doesn't exist in watchers map",
			name,
		)
		panic(errString)
	}
	return val
}
