package k8s

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// EndpointsCache is a simple cache of endpoints.
// It allows callers to quickly get given endpoints in a given
// namespace, or watch for changes to specific endpoints, all
// without incurring the cost of issuing a network request
// to the Kubernetes API
type EndpointsCache interface {
	json.Marshaler
	// Get gets the endpoints with the given name
	// in the given namespace from the cache.
	//
	// If the endpoints doesn't exist in the cache, it
	// will be requested from the backing store (most commonly
	// the Kubernetes API server)
	Get(namespace, name string) (v1.Endpoints, error)
	// Watch opens a watch stream for the endpoints with
	// the given name in the given namespace from the cache.
	//
	// If the endpoints don't exist in the cache, it
	// will be requested from the backing store (most commonly
	// the Kubernetes API server)
	Watch(namespace, name string) (watch.Interface, error)
}
