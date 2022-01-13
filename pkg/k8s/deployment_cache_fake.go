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
	mut      *sync.RWMutex
	current  map[string]appsv1.Deployment
	watchers map[string]*watch.RaceFreeFakeWatcher
}

var _ DeploymentCache = &FakeDeploymentCache{}

func NewFakeDeploymentCache() *FakeDeploymentCache {
	return &FakeDeploymentCache{
		mut:      &sync.RWMutex{},
		current:  make(map[string]appsv1.Deployment),
		watchers: make(map[string]*watch.RaceFreeFakeWatcher),
	}
}

// AddDeployment adds a deployment to the current in-memory cache without
// sending an event to any of the watchers
func (f *FakeDeploymentCache) AddDeployment(depl appsv1.Deployment) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.current[key(depl.Namespace, depl.Name)] = depl
}

// CurrentDeployments returns a map of all the current deployments.
//
// The key in the map is a combination of the namespace and name of
// the corresponding deployment, but the format of the key is not guaranteed
func (f *FakeDeploymentCache) CurrentDeployments() map[string]appsv1.Deployment {
	f.mut.RLock()
	defer f.mut.RUnlock()
	return f.current
}

// GetWatcher gets the watcher for the given namespace and name, or
// nil if there wasn't one registered.
//
// Watchers are registered by the .Watch() method
func (f *FakeDeploymentCache) GetWatcher(ns, name string) *watch.RaceFreeFakeWatcher {
	f.mut.RLock()
	defer f.mut.RUnlock()
	return f.watchers[key(ns, name)]
}

func (f *FakeDeploymentCache) MarshalJSON() ([]byte, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	ret := map[string]int32{}
	for name, deployment := range f.current {
		ret[name] = *deployment.Spec.Replicas
	}
	return json.Marshal(ret)
}

func (f *FakeDeploymentCache) Get(ns string, name string) (appsv1.Deployment, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	ret, ok := f.current[key(ns, name)]
	if ok {
		return ret, nil
	}
	return appsv1.Deployment{}, fmt.Errorf("no deployment %s found", name)
}

func (f *FakeDeploymentCache) Watch(ns, name string) watch.Interface {
	f.mut.RLock()
	defer f.mut.RUnlock()
	watcher, ok := f.watchers[key(ns, name)]
	if !ok {
		watcher = watch.NewRaceFreeFake()
		f.watchers[key(ns, name)] = watcher
	}
	return watcher
}

func (f *FakeDeploymentCache) Set(ns, name string, deployment appsv1.Deployment) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.current[key(ns, name)] = deployment
}

// SetWatcher creates a new race-free fake watcher and sets it into
// the internal watchers map. After this call, a call to Watch() with
// the same namespace and name values will return a valid
// watcher
func (f *FakeDeploymentCache) SetWatcher(ns, name string) *watch.RaceFreeFakeWatcher {
	f.mut.Lock()
	defer f.mut.Unlock()
	watcher := watch.NewRaceFreeFake()
	f.watchers[key(ns, name)] = watcher
	return watcher
}

func (f *FakeDeploymentCache) SetReplicas(ns, name string, num int32) error {
	f.mut.Lock()
	defer f.mut.Unlock()
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
