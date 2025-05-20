package k8s

import (
	"encoding/json"
	"fmt"
	"sync"

	discov1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// FakeEndpointsCache is a fake implementation of
// EndpointsCache, suitable for testing interceptor-level
// logic, without requiring any real Kubernetes client
// or API interaction
type FakeEndpointsCache struct {
	json.Marshaler
	mut      *sync.RWMutex
	current  map[string]discov1.EndpointSlice
	watchers map[string]*watch.RaceFreeFakeWatcher
}

var _ EndpointsCache = &FakeEndpointsCache{}

func NewFakeEndpointsCache() *FakeEndpointsCache {
	return &FakeEndpointsCache{
		mut:      &sync.RWMutex{},
		current:  make(map[string]discov1.EndpointSlice),
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
		for _, subset := range endpoints.Endpoints {
			total += len(subset.Addresses)
		}
		ret[name] = total
	}
	return json.Marshal(ret)
}

func (f *FakeEndpointsCache) Get(ns string, name string) (discov1.EndpointSlice, error) {
	f.mut.RLock()
	defer f.mut.RUnlock()
	ret, ok := f.current[key(ns, name)]
	if ok {
		return ret, nil
	}
	return discov1.EndpointSlice{}, fmt.Errorf("no endpoints %s found", name)
}

// Set adds a endpoints to the current in-memory cache without
// sending an event to any of the watchers
func (f *FakeEndpointsCache) Set(endp discov1.EndpointSlice) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.current[key(endp.Namespace, endp.GenerateName)] = endp
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

func (f *FakeEndpointsCache) SetEndpoints(ns, name string, num int) error {
	endpointSlice, err := f.Get(ns, name)
	if err != nil {
		return fmt.Errorf("no endpoints %s found", name)
	}
	for i := 0; i < num; i++ {
		endpoint := discov1.Endpoint{
			Addresses: []string{
				"1.2.3.4",
			},
		}
		endpointSlice.Endpoints = append(endpointSlice.Endpoints, endpoint)
	}
	f.Set(endpointSlice)
	return nil
}

func key(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}
