package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
)

// ServiceCache is an interface for caching service objects
type ServiceCache interface {
	// Get gets a service with the given namespace and name from the cache
	// If the service doesn't exist in the cache, it will be fetched from the API server
	Get(ctx context.Context, namespace, name string) (*v1.Service, error)
}

// InformerBackedServicesCache is a cache of services backed by a shared informer
type InformerBackedServicesCache struct {
	lggr      logr.Logger
	cl        kubernetes.Interface
	svcLister listerv1.ServiceLister
}

// FakeServiceCache is a fake implementation of a ServiceCache for testing
type FakeServiceCache struct {
	current map[string]v1.Service
	mut     sync.RWMutex
}

// NewInformerBackedServiceCache creates a new InformerBackedServicesCache
func NewInformerBackedServiceCache(lggr logr.Logger, cl kubernetes.Interface, factory informers.SharedInformerFactory) *InformerBackedServicesCache {
	return &InformerBackedServicesCache{
		lggr:      lggr.WithName("InformerBackedServicesCache"),
		cl:        cl,
		svcLister: factory.Core().V1().Services().Lister(),
	}
}

// Get gets a service with the given namespace and name from the cache and as a fallback from the API server
func (c *InformerBackedServicesCache) Get(ctx context.Context, namespace, name string) (*v1.Service, error) {
	svc, err := c.svcLister.Services(namespace).Get(name)
	if err == nil {
		c.lggr.V(1).Info("Service found in cache", "namespace", namespace, "name", name)
		return svc, nil
	}
	c.lggr.V(1).Info("Service not found in cache, fetching from API server", "namespace", namespace, "name", name, "error", err)
	return c.cl.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

// NewFakeServiceCache creates a new FakeServiceCache
func NewFakeServiceCache() *FakeServiceCache {
	return &FakeServiceCache{current: make(map[string]v1.Service)}
}

// Get gets a service with the given namespace and name from the cache
func (c *FakeServiceCache) Get(_ context.Context, namespace, name string) (*v1.Service, error) {
	c.mut.RLock()
	defer c.mut.RUnlock()
	svc, ok := c.current[key(namespace, name)]
	if !ok {
		return nil, fmt.Errorf("service not found")
	}
	return &svc, nil
}

// Add adds a service to the cache
func (c *FakeServiceCache) Add(svc v1.Service) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.current[key(svc.Namespace, svc.Name)] = svc
}
