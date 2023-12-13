package k8s

import (
	"encoding/json"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// FakeEndpointsCache is a fake implementation of
// EndpointsCache, suitable for testing interceptor-level
// logic, without requiring any real Kubernetes client
// or API interaction
type FakeEndpointsCache struct {
	json.Marshaler
	mut      *sync.RWMutex
	current  map[string]v1.Endpoints
	watchers map[string]*watch.RaceFreeFakeWatcher
}

var _ EndpointsCache = &FakeEndpointsCache{}

func NewFakeEndpointsCache() *FakeEndpointsCache {
	return &FakeEndpointsCache{
		mut:      &sync.RWMutex{},
		current:  make(map[string]v1.Endpoints),
		watchers: make(map[string]*watch.RaceFreeFakeWatcher),
	}
}

// GetWatcher gets the watcher for the given namespace and name, or
// nil if there wasn't one registered.
//
// Watchers are registered by the .Watch() method
func (f *FakeEndpointsCache) GetWatcher(ns, name string) *watch.RaceFreeFakeWatcher {
	f.mut.RLock()
	defer f.mut.RUnlock()
	return f.watchers[key(ns, name)]
}

func (f *FakeEndpointsCache) MarshalJSON() ([]byte, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	ret := map[string]int{}
	for name, endpoints := range f.current {
		total := 0
		for _, subset := range endpoints.Subsets {
			total += len(subset.Addresses)
		}
		ret[name] = total
	}
	return json.Marshal(ret)
}

func (f *FakeEndpointsCache) Get(ns string, name string) (v1.Endpoints, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	ret, ok := f.current[key(ns, name)]
	if ok {
		return ret, nil
	}
	return v1.Endpoints{}, fmt.Errorf("no endpoints %s found", name)
}

// Set adds a endpoints to the current in-memory cache without
// sending an event to any of the watchers
func (f *FakeEndpointsCache) Set(endp v1.Endpoints) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.current[key(endp.Namespace, endp.Name)] = endp
}

func (f *FakeEndpointsCache) Watch(ns, name string) (watch.Interface, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	watcher, ok := f.watchers[key(ns, name)]
	if !ok {
		watcher = watch.NewRaceFreeFake()
		f.watchers[key(ns, name)] = watcher
	}
	return watcher, nil
}

// SetWatcher creates a new race-free fake watcher and sets it into
// the internal watchers map. After this call, a call to Watch() with
// the same namespace and name values will return a valid
// watcher
func (f *FakeEndpointsCache) SetWatcher(ns, name string) *watch.RaceFreeFakeWatcher {
	f.mut.Lock()
	defer f.mut.Unlock()
	watcher := watch.NewRaceFreeFake()
	f.watchers[key(ns, name)] = watcher
	return watcher
}

func (f *FakeEndpointsCache) SetSubsets(ns, name string, num int) error {
	endpoints, err := f.Get(ns, name)
	if err != nil {
		return fmt.Errorf("no endpoints %s found", name)
	}
	subsets := []v1.EndpointSubset{}

	for i := 0; i < num; i++ {
		subset := v1.EndpointSubset{
			Addresses: []v1.EndpointAddress{
				{
					IP: "1.2.3.4",
				},
			},
		}
		subsets = append(subsets, subset)
	}

	endpoints.Subsets = subsets
	f.Set(endpoints)
	return nil
}

func key(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
