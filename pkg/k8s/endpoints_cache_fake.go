package k8s

import (
	"encoding/json"
	"fmt"
	"sync"

	discov1 "k8s.io/api/discovery/v1"
)

// FakeEndpointsCache is a fake implementation of
// EndpointsCache, suitable for testing interceptor-level
// logic, without requiring any real Kubernetes client
// or API interaction
type FakeEndpointsCache struct {
	json.Marshaler
	mut         *sync.RWMutex
	current     map[string]discov1.EndpointSlice
	mu          sync.Mutex
	subscribers map[string]chan struct{}
}

var _ EndpointsCache = &FakeEndpointsCache{}

func NewFakeEndpointsCache() *FakeEndpointsCache {
	return &FakeEndpointsCache{
		mut:         &sync.RWMutex{},
		current:     make(map[string]discov1.EndpointSlice),
		subscribers: make(map[string]chan struct{}),
	}
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

// Subscribe returns a buffered channel that receives a signal when
// the given service's endpoints change.
func (f *FakeEndpointsCache) Subscribe(ns, serviceName string) <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := key(ns, serviceName)
	if ch, ok := f.subscribers[k]; ok {
		return ch
	}
	ch := make(chan struct{}, 1)
	f.subscribers[k] = ch
	return ch
}

// Notify sends a non-blocking signal to the subscriber for the given
// service, if any.
func (f *FakeEndpointsCache) Notify(ns, serviceName string) {
	f.mu.Lock()
	ch, ok := f.subscribers[key(ns, serviceName)]
	f.mu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Set adds a endpoints to the current in-memory cache without
// sending an event to any of the watchers
func (f *FakeEndpointsCache) Set(endp discov1.EndpointSlice) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.current[key(endp.Namespace, endp.GenerateName)] = endp
}

func (f *FakeEndpointsCache) SetEndpoints(ns, name string, num int) error {
	endpointSlice, err := f.Get(ns, name)
	if err != nil {
		return fmt.Errorf("no endpoints %s found", name)
	}
	for range num {
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
