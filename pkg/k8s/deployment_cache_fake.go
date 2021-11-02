package k8s

import (
	"encoding/json"
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
	return json.Marshal(f.Current)
}

func (f *FakeDeploymentCache) Get(ns string, name string) (appsv1.Deployment, error) {
	f.Mut.RLock()
	defer f.Mut.RUnlock()
	ret, ok := f.Current[key(ns, name)]
	if ok {
		return ret, nil
	}
	return appsv1.Deployment{}, fmt.Errorf("no deployment %s found", name)
}

func (f *FakeDeploymentCache) Watch(ns, name string) watch.Interface {
	f.Mut.RLock()
	defer f.Mut.RUnlock()
	watcher, ok := f.Watchers[key(ns, name)]
	if !ok {
		return watch.NewRaceFreeFake()
	}
	return watcher
}

func (f *FakeDeploymentCache) Set(ns, name string, deployment appsv1.Deployment) {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	f.Current[key(ns, name)] = deployment
}

// SetWatcher creates a new race-free fake watcher and sets it into
// the internal watchers map. After this call, a call to Watch() with
// the same namespace and name values will return a valid
// watcher
func (f *FakeDeploymentCache) SetWatcher(ns, name string) *watch.RaceFreeFakeWatcher {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	watcher := watch.NewRaceFreeFake()
	f.Watchers[key(ns, name)] = watcher
	return watcher
}

func (f *FakeDeploymentCache) SetReplicas(ns, name string, num int32) error {
	f.Mut.Lock()
	defer f.Mut.Unlock()
	deployment, err := f.Get(ns, name)
	if err != nil {
		return fmt.Errorf("no deployment %s found", name)
	}
	deployment.Spec.Replicas = &num
	return nil
}

func key(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
