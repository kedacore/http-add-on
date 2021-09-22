package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
	// 	"context"
	// 	"encoding/json"
	// 	"fmt"
	// 	"sync"
	// 	"time"
	// 	"github.com/go-logr/logr"
	// 	"github.com/pkg/errors"
)

// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/watch"

// // DeploymentCache is a simple cache of deployments.
type DeploymentCache interface {
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
	Watch(namespace, name string) watch.Interface
}
