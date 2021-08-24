package k8s

import (
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type FakeDeploymentCache struct {
	Mut      *sync.RWMutex
	Current  map[string]appsv1.Deployment
	Watchers map[string]*watch.RaceFreeFakeWatcher
}

func NewFakeDeploymentCache() *FakeDeploymentCache {
	return &FakeDeploymentCache{
		Mut:      &sync.RWMutex{},
		Current:  make(map[string]appsv1.Deployment),
		Watchers: make(map[string]*watch.RaceFreeFakeWatcher),
	}
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
