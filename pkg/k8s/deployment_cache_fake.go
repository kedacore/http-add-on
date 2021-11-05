package k8s

import (
	"encoding/json"
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// FakeDeploymentCache is a fake implementation of
// DeploymentCache, suitable for testing interceptor-level
// logic, without requiring any real Kubernetes client
// or API interaction
type FakeDeploymentCache struct {
	json.Marshaler
	Mut      *sync.RWMutex
	Current  map[string]appsv1.Deployment
	Watchers map[string]*watch.RaceFreeFakeWatcher
}

var _ DeploymentCache = &FakeDeploymentCache{}

func NewFakeDeploymentCache() *FakeDeploymentCache {
	return &FakeDeploymentCache{
		Mut:      &sync.RWMutex{},
		Current:  make(map[string]appsv1.Deployment),
		Watchers: make(map[string]*watch.RaceFreeFakeWatcher),
	}
}

func (f *FakeDeploymentCache) MarshalJSON() ([]byte, error) {
	f.Mut.RLock()
	defer f.Mut.RUnlock()
	ret := map[string]int32{}
	for name, deployment := range f.Current {
		ret[name] = *deployment.Spec.Replicas
	}
	return json.Marshal(ret)
}

func (f *FakeDeploymentCache) Get(name string) (appsv1.Deployment, error) {
	f.Mut.RLock()
	defer f.Mut.RUnlock()
	ret, ok := f.Current[name]
	if ok {
		return ret, nil
	}
	return appsv1.Deployment{}, fmt.Errorf("no deployment %s found", name)
}

func (f *FakeDeploymentCache) Watch(name string) watch.Interface {
	f.Mut.RLock()
	defer f.Mut.RUnlock()
	watcher, ok := f.Watchers[name]
	if !ok {
		return watch.NewRaceFreeFake()
	}
	return watcher
}

func (f *FakeDeploymentCache) Set(name string, deployment appsv1.Deployment) {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	f.Current[name] = deployment
}

func (f *FakeDeploymentCache) SetWatcher(name string) *watch.RaceFreeFakeWatcher {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	watcher := watch.NewRaceFreeFake()
	f.Watchers[name] = watcher
	return watcher
}

func (f *FakeDeploymentCache) SetReplicas(name string, num int32) error {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	deployment, err := f.Get(name)
	if err != nil {
		return fmt.Errorf("no deployment %s found", name)
	}
	deployment.Spec.Replicas = &num
	return nil
}
