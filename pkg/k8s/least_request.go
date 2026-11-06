package k8s

import (
	"fmt"
	"sync"
)

// LoadBalancing selects how a ready pod is picked for direct-pod routing.
type LoadBalancing string

const (
	// LoadBalancingRandom picks a random ready pod.
	LoadBalancingRandom LoadBalancing = "random"
	// LoadBalancingLeastRequest picks two random ready pods and uses the one
	// with fewer in-flight requests from this interceptor replica.
	LoadBalancingLeastRequest LoadBalancing = "least-request"
)

// ParseLoadBalancing parses a load balancing mode. An empty string selects
// LoadBalancingRandom.
func ParseLoadBalancing(s string) (LoadBalancing, error) {
	switch LoadBalancing(s) {
	case "", LoadBalancingRandom:
		return LoadBalancingRandom, nil
	case LoadBalancingLeastRequest:
		return LoadBalancingLeastRequest, nil
	default:
		return "", fmt.Errorf("invalid load balancing mode %q, must be %q or %q", s, LoadBalancingRandom, LoadBalancingLeastRequest)
	}
}

// inFlightRequests counts the requests in flight per pod host. Hosts are
// removed when their count drops to zero, so pods that went away don't keep
// an entry.
type inFlightRequests struct {
	mu     sync.Mutex
	counts map[string]int
}

func newInFlightRequests() *inFlightRequests {
	return &inFlightRequests{counts: make(map[string]int)}
}

func (f *inFlightRequests) count(host string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[host]
}

// start marks a request to host as in flight until the returned function is
// called.
func (f *inFlightRequests) start(host string) func() {
	f.mu.Lock()
	f.counts[host]++
	f.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.counts[host]--; f.counts[host] <= 0 {
				delete(f.counts, host)
			}
		})
	}
}
