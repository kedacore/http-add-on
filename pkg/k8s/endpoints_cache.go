package k8s

import (
	"encoding/json"

	discov1 "k8s.io/api/discovery/v1"
)

// EndpointsCache is a simple cache of endpoints.
// It allows callers to quickly get given endpoints in a given
// namespace, or be notified of changes to endpoints, all
// without incurring the cost of issuing a network request
// to the Kubernetes API
type EndpointsCache interface {
	json.Marshaler
	// Get gets the endpoints with the given name
	// in the given namespace from the cache.
	Get(namespace, name string) (discov1.EndpointSlice, error)
	// Subscribe returns a channel that receives a signal whenever
	// an EndpointSlice owned by the given service changes. The
	// channel is buffered (capacity 1) so a notification that
	// arrives while the caller is busy is coalesced and not lost.
	Subscribe(namespace, serviceName string) <-chan struct{}
}
