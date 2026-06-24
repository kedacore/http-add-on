package k8s

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// endpoint is one ready pod: its address paired with the container port from
// the same EndpointSlice.
type endpoint struct {
	ip   string
	port int32
}

// serviceState is an immutable snapshot of a service's ready pods, swapped
// atomically on every EndpointSlice event so readers need no locks.
type serviceState struct {
	ready bool // true if the service has at least one ready endpoint
	// candidates maps portName ("" for unnamed) to its ready pods.
	candidates map[string][]endpoint
}

func (s *serviceState) hasReady() bool { return s != nil && s.ready }

// ReadyEndpointsCache maintains a derived map of service -> serviceState
// for O(1) hot-path lookups, plus a broadcast mechanism so the cold-start
// wait function can block until a service becomes ready.
type ReadyEndpointsCache struct {
	lggr logr.Logger

	// "namespace/service" -> *serviceState
	states sync.Map

	// Broadcast mechanism: the channel is closed on a ready-state transition
	// (ready→not-ready or not-ready→ready), then replaced with a fresh one.
	// Waiters select on the channel.
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
	if v, ok := c.states.Load(serviceKey); ok {
		return v.(*serviceState).hasReady()
	}
	return false
}

// WaitForReady waits until the service has at least one ready endpoint or
// the context is cancelled/timed out.
//
// Returns:
//   - (false, podHost, nil)  — warm backend, already ready (fast path)
//   - (true, podHost, nil)   — cold start, backend became ready
//   - (false, "", error)     — context cancelled or timed out
//
// podHost is "ip:port" for portName, or "" when portName has no candidates
// (signals direct-pod routing isn't possible).
func (c *ReadyEndpointsCache) WaitForReady(ctx context.Context, serviceKey, portName string) (isColdStart bool, podHost string, err error) {
	if v, ok := c.states.Load(serviceKey); ok {
		if state := v.(*serviceState); state.hasReady() {
			return false, pickHost(state, portName), nil
		}
	}

	// Get the current notification channel before re-checking
	c.mu.Lock()
	ch := c.notifyCh
	c.mu.Unlock()

	// Re-check after getting the channel (close the race window).
	// Return isColdStart=false: we never actually blocked, so this
	// is still the warm/fast path.
	if v, ok := c.states.Load(serviceKey); ok {
		if state := v.(*serviceState); state.hasReady() {
			return false, pickHost(state, portName), nil
		}
	}

	c.lggr.V(1).Info("cold-start: waiting for ready endpoints", "key", serviceKey)

	for {
		select {
		case <-ctx.Done():
			return false, "", fmt.Errorf(
				"context done while waiting for ready endpoints for %s: %w",
				serviceKey, ctx.Err(),
			)

		case <-ch:
			if v, ok := c.states.Load(serviceKey); ok {
				if state := v.(*serviceState); state.hasReady() {
					c.lggr.Info("cold-start: endpoints became ready", "key", serviceKey)
					return true, pickHost(state, portName), nil
				}
			}
			// Not our service — get the new channel and re-check
			// before waiting again to avoid missing a broadcast
			// that fired between the check above and now.
			c.mu.Lock()
			ch = c.notifyCh
			c.mu.Unlock()
			if v, ok := c.states.Load(serviceKey); ok {
				if state := v.(*serviceState); state.hasReady() {
					c.lggr.Info("cold-start: endpoints became ready", "key", serviceKey)
					return true, pickHost(state, portName), nil
				}
			}
		}
	}
}

// pickHost selects a random ready pod for portName from state and returns its
// "ip:port" host string. Returns "" if portName has no candidates.
func pickHost(state *serviceState, portName string) string {
	eps := state.candidates[portName]
	if len(eps) == 0 {
		return ""
	}
	ep := eps[rand.IntN(len(eps))] //nolint:gosec // G404: math/rand is sufficient for load-balancing endpoint selection
	return net.JoinHostPort(ep.ip, strconv.Itoa(int(ep.port)))
}

// Update checks the given EndpointSlices, builds a new serviceState snapshot,
// and atomically replaces the old one. serviceKey is "namespace/service".
func (c *ReadyEndpointsCache) Update(serviceKey string, endpointSlices []*discov1.EndpointSlice) {
	if len(endpointSlices) == 0 {
		if v, ok := c.states.LoadAndDelete(serviceKey); ok {
			if v.(*serviceState).hasReady() {
				c.broadcast()
			}
		}
		return
	}

	newState := collectServiceState(endpointSlices)
	old, loaded := c.states.Swap(serviceKey, newState)

	var oldState *serviceState
	if loaded {
		oldState = old.(*serviceState)
	}

	if oldState.hasReady() != newState.hasReady() {
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

// collectServiceState builds an immutable serviceState snapshot. Each candidate
// pairs a ready pod's address with a port from the same slice, keyed by portName
// ("" for unnamed). Address families (IPv4/IPv6) are not distinguished: in
// dual-stack clusters, candidates from both families are pooled together and
// pickHost may return an IPv6 address even if the caller can only reach IPv4
// targets (or vice versa). This is acceptable for single-stack clusters (the
// common case) and a known limitation for dual-stack deployments.
func collectServiceState(slices []*discov1.EndpointSlice) *serviceState {
	// Dedup (ip, port) per portName so overlapping slices don't weight a pod twice.
	seen := make(map[string]map[endpoint]struct{})
	candidates := make(map[string][]endpoint)
	anyReady := false

	for _, sl := range slices {
		// Collect this slice's ports.
		type slicePort struct {
			name string
			port int32
		}
		var slicePorts []slicePort
		for _, p := range sl.Ports {
			if p.Port == nil {
				continue
			}
			name := ""
			if p.Name != nil {
				name = *p.Name
			}
			slicePorts = append(slicePorts, slicePort{name, *p.Port})
		}

		// Collect this slice's ready pod IPs (the canonical address per pod).
		var readyIPs []string
		for i := range sl.Endpoints {
			ep := &sl.Endpoints[i]
			// Kubernetes guarantees that Ready is false for terminating pods, so a
			// separate Terminating check is unnecessary. For services with
			// publishNotReadyAddresses, Ready is always true — we respect that.
			if (ep.Conditions.Ready == nil || *ep.Conditions.Ready) && len(ep.Addresses) > 0 {
				readyIPs = append(readyIPs, ep.Addresses[0])
				anyReady = true
			}
		}

		if len(readyIPs) == 0 || len(slicePorts) == 0 {
			continue
		}

		// Cross-product: pair every ready pod with every port from this slice.
		for _, sp := range slicePorts {
			if seen[sp.name] == nil {
				seen[sp.name] = make(map[endpoint]struct{})
			}
			for _, ip := range readyIPs {
				k := endpoint{ip: ip, port: sp.port}
				if _, dup := seen[sp.name][k]; dup {
					continue
				}
				seen[sp.name][k] = struct{}{}
				candidates[sp.name] = append(candidates[sp.name], k)
			}
		}
	}

	return &serviceState{
		ready:      anyReady,
		candidates: candidates,
	}
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
