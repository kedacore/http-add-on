package k8s

import (
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// DeploymentCache is a simple cache of deployments.
// It allows callers to quickly get a given deployment in a given
// namespace, or watch for changes to a specific deployment, all
// without incurring the cost of issuing a network request
// to the Kubernetes API
type DeploymentCache interface {
	json.Marshaler
	// Get gets the a deployment with the given name
	// in the given namespace from the cache.
	//
	// If the deployment doesn't exist in the cache, it
	// will be requested from the backing store (most commonly
	// the Kubernetes API server)
	Get(namespace, name string) (appsv1.Deployment, error)
	// Watch opens a watch stream for the deployment with
	// the given name in the given namespace from the cache.
	//
	// If the deployment doesn't exist in the cache, it
	// will be requested from the backing store (most commonly
	// the Kubernetes API server)
	Watch(namespace, name string) (watch.Interface, error)
}
