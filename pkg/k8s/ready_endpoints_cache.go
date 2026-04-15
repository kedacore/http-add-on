package k8s

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceEndpoints holds the set of ready pod addresses and their port
// mappings for a single Kubernetes Service. Instances are immutable after
// creation; updates produce a new snapshot that is atomically swapped in.
type ServiceEndpoints struct {
	Addresses []string         // deduplicated ready pod IPs
	Ports     map[string]int32 // portName -> containerPort ("" for unnamed)
}

// ReadyEndpointsCache maintains a derived map of service -> ServiceEndpoints
// for O(1) hot-path lookups, plus a broadcast mechanism so the cold-start
// wait function can block until a service becomes ready.
type ReadyEndpointsCache struct {
	lggr logr.Logger

	// "namespace/service" -> *atomic.Pointer[ServiceEndpoints]
	endpoints sync.Map

	// Broadcast mechanism: the channel is closed on any change,
	// then replaced with a fresh one. Waiters select on the channel.
	mu       sync.Mutex
	notifyCh chan struct{}
}

// NewReadyEndpointsCache creates a new empty ready endpoints cache.
func NewReadyEndpointsCache(lggr logr.Logger) *ReadyEndpointsCache {
	return &ReadyEndpointsCache{
		lggr:     lggr.WithName("readyEndpointsCache"),
		notifyCh: make(chan struct{}),
	}
}

// HasReadyEndpoints returns true if the service has at least one ready endpoint.
// This is the fast hot-path check (one atomic load).
func (c *ReadyEndpointsCache) HasReadyEndpoints(serviceKey string) bool {
	if v, ok := c.endpoints.Load(serviceKey); ok {
		se := v.(*atomic.Pointer[ServiceEndpoints]).Load()
		return se != nil && len(se.Addresses) > 0
	}
	return false
}

// GetEndpoints returns the current endpoint snapshot for the given service,
// or nil if the service is unknown or has no endpoint slices.
func (c *ReadyEndpointsCache) GetEndpoints(serviceKey string) *ServiceEndpoints {
	if v, ok := c.endpoints.Load(serviceKey); ok {
		return v.(*atomic.Pointer[ServiceEndpoints]).Load()
	}
	return nil
}

// WaitForReady waits until the service has at least one ready endpoint or
// the context is cancelled/timed out.
// Returns:
//   - (false, nil)  — warm backend, already ready (fast path)
//   - (true, nil)   — cold start, but backend became ready
//   - (false, error) — context cancelled or timed out
func (c *ReadyEndpointsCache) WaitForReady(ctx context.Context, serviceKey string) (isColdStart bool, err error) {
	if c.HasReadyEndpoints(serviceKey) {
		return false, nil
	}

	// Get the current notification channel before re-checking
	c.mu.Lock()
	ch := c.notifyCh
	c.mu.Unlock()

	// Re-check after getting the channel (close the race window).
	// Return isColdStart=false: we never actually blocked, so this
	// is still the warm/fast path.
	if c.HasReadyEndpoints(serviceKey) {
		return false, nil
	}

	c.lggr.V(1).Info("cold-start: waiting for ready endpoints", "key", serviceKey)

	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf(
				"context done while waiting for ready endpoints for %s: %w",
				serviceKey, ctx.Err(),
			)

		case <-ch:
			if c.HasReadyEndpoints(serviceKey) {
				c.lggr.Info("cold-start: endpoints became ready", "key", serviceKey)
				return true, nil
			}
			// Not our service — get the new channel and re-check
			// before waiting again to avoid missing a broadcast
			// that fired between the check above and now.
			c.mu.Lock()
			ch = c.notifyCh
			c.mu.Unlock()
			if c.HasReadyEndpoints(serviceKey) {
				c.lggr.Info("cold-start: endpoints became ready", "key", serviceKey)
				return true, nil
			}
		}
	}
}

// Update rebuilds the endpoint snapshot for the given service from the
// supplied EndpointSlices. If no slices remain, the key is removed to
// avoid unbounded map growth.
func (c *ReadyEndpointsCache) Update(serviceKey string, endpointSlices []*discov1.EndpointSlice) {
	if len(endpointSlices) == 0 {
		c.endpoints.Delete(serviceKey)
		c.broadcast()
		return
	}

	se := buildServiceEndpoints(endpointSlices)

	p := &atomic.Pointer[ServiceEndpoints]{}
	v, loaded := c.endpoints.LoadOrStore(serviceKey, p)
	if loaded {
		p = v.(*atomic.Pointer[ServiceEndpoints])
	}

	old := p.Swap(se)
	oldReady := old != nil && len(old.Addresses) > 0
	newReady := len(se.Addresses) > 0
	if oldReady != newReady {
		c.broadcast()
	}
}

// broadcast wakes all waiting goroutines by closing the current channel
// and replacing it with a new one.
func (c *ReadyEndpointsCache) broadcast() {
	c.mu.Lock()
	old := c.notifyCh
	c.notifyCh = make(chan struct{})
	c.mu.Unlock()
	close(old)
}

// buildServiceEndpoints collects all ready endpoint addresses and port
// mappings from the given EndpointSlices into an immutable snapshot.
func buildServiceEndpoints(slices []*discov1.EndpointSlice) *ServiceEndpoints {
	addrSet := make(map[string]struct{})
	ports := make(map[string]int32)

	for _, slice := range slices {
		for _, p := range slice.Ports {
			name := ""
			if p.Name != nil {
				name = *p.Name
			}
			if p.Port != nil {
				ports[name] = *p.Port
			}
		}

		for i := range slice.Endpoints {
			ep := &slice.Endpoints[i]
			if (ep.Conditions.Ready == nil || *ep.Conditions.Ready) && len(ep.Addresses) > 0 {
				for _, addr := range ep.Addresses {
					addrSet[addr] = struct{}{}
				}
			}
		}
	}

	addresses := make([]string, 0, len(addrSet))
	for addr := range addrSet {
		addresses = append(addresses, addr)
	}

	return &ServiceEndpoints{
		Addresses: addresses,
		Ports:     ports,
	}
}

// hasAnyReadyEndpoint returns true if the slice contains at least one
// ready endpoint with at least one address.
// Kubernetes guarantees that Ready is false for terminating pods, so a
// separate Terminating check is unnecessary. For services with
// publishNotReadyAddresses, Ready is always true — we respect that.
func hasAnyReadyEndpoint(slice *discov1.EndpointSlice) bool {
	for i := range slice.Endpoints {
		ep := &slice.Endpoints[i]
		if (ep.Conditions.Ready == nil || *ep.Conditions.Ready) && len(ep.Addresses) > 0 {
			return true
		}
	}
	return false
}

// updateReadyCache updates the ready cache for the service that owns
// the given EndpointSlice.
func updateReadyCache(lggr logr.Logger, ctrlCache ctrlcache.Cache, readyCache *ReadyEndpointsCache, slice *discov1.EndpointSlice) {
	svcName, ok := slice.Labels[discov1.LabelServiceName]
	if !ok || svcName == "" {
		return
	}
	ns := slice.Namespace

	list := &discov1.EndpointSliceList{}
	if err := ctrlCache.List(context.Background(), list,
		client.InNamespace(ns),
		client.MatchingLabels{discov1.LabelServiceName: svcName},
	); err != nil {
		lggr.Error(err, "failed to list endpoint slices for ready cache update",
			"namespace", ns, "service", svcName)
		return
	}

	allSlices := make([]*discov1.EndpointSlice, len(list.Items))
	for idx := range list.Items {
		allSlices[idx] = &list.Items[idx]
	}
	readyCache.Update(ns+"/"+svcName, allSlices)
}

func addEvtHandler(lggr logr.Logger, ctrlCache ctrlcache.Cache, readyCache *ReadyEndpointsCache, obj any) {
	eps, ok := obj.(*discov1.EndpointSlice)
	if !ok {
		lggr.Error(
			fmt.Errorf("informer expected EndpointSlice, got %v", obj),
			"skipping event",
		)
		return
	}

	updateReadyCache(lggr, ctrlCache, readyCache, eps)
}

func updateEvtHandler(lggr logr.Logger, ctrlCache ctrlcache.Cache, readyCache *ReadyEndpointsCache, _, newObj any) {
	eps, ok := newObj.(*discov1.EndpointSlice)
	if !ok {
		lggr.Error(
			fmt.Errorf("informer expected EndpointSlice, got %v", newObj),
			"skipping event",
		)
		return
	}

	updateReadyCache(lggr, ctrlCache, readyCache, eps)
}

func deleteEvtHandler(lggr logr.Logger, ctrlCache ctrlcache.Cache, readyCache *ReadyEndpointsCache, obj any) {
	eps, err := endpointSliceFromDeleteObj(obj)
	if err != nil {
		lggr.Error(
			err,
			"skipping event",
		)
		return
	}

	updateReadyCache(lggr, ctrlCache, readyCache, eps)
}

// endpointSliceFromDeleteObj unwraps EndpointSlice delete events from either
// a direct object or a DeletedFinalStateUnknown tombstone.
func endpointSliceFromDeleteObj(obj any) (*discov1.EndpointSlice, error) {
	switch t := obj.(type) {
	case *discov1.EndpointSlice:
		return t, nil
	case cache.DeletedFinalStateUnknown:
		eps, ok := t.Obj.(*discov1.EndpointSlice)
		if !ok {
			return nil, fmt.Errorf("informer expected EndpointSlice in tombstone, got %T", t.Obj)
		}
		return eps, nil
	default:
		return nil, fmt.Errorf("informer expected EndpointSlice, got %T", obj)
	}
}

func NewReadyEndpointsCacheWithInformer(
	ctx context.Context,
	lggr logr.Logger,
	ctrlCache ctrlcache.Cache,
) (*ReadyEndpointsCache, error) {
	informer, err := ctrlCache.GetInformer(ctx, &discov1.EndpointSlice{})
	if err != nil {
		lggr.Error(err, "error getting EndpointSlice informer from controller-runtime cache")
		return nil, err
	}

	readyCache := NewReadyEndpointsCache(lggr)

	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			addEvtHandler(lggr, ctrlCache, readyCache, obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			updateEvtHandler(lggr, ctrlCache, readyCache, oldObj, newObj)
		},
		DeleteFunc: func(obj any) {
			deleteEvtHandler(lggr, ctrlCache, readyCache, obj)
		},
	})
	if err != nil {
		lggr.Error(err, "error adding event handler to EndpointSlice informer")
		return nil, err
	}

	return readyCache, nil
}
