package k8s

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
)

// ReadyEndpointsCache maintains a derived map of service -> ready (bool)
// for O(1) hot-path lookups, plus a broadcast mechanism so the cold-start
// wait function can block until a service becomes ready.
//
// Hot path (warm backend): single atomic load — no locks, no allocations.
// Cold path (scale-from-zero): subscribe to broadcast channel, wait for notification.
type ReadyEndpointsCache struct {
	lggr logr.Logger

	// "namespace/service" -> *atomic.Bool
	ready sync.Map

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
	if v, ok := c.ready.Load(serviceKey); ok {
		return v.(*atomic.Bool).Load()
	}
	return false
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

	// Re-check after getting the channel (close the race window)
	if c.HasReadyEndpoints(serviceKey) {
		return true, nil
	}

	c.lggr.V(1).Info("cold-start: waiting for ready endpoints", "key", serviceKey)

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case <-ch:
			if c.HasReadyEndpoints(serviceKey) {
				c.lggr.Info("cold-start: endpoints became ready", "key", serviceKey)
				return true, nil
			}
			// Not our service — get the new channel and wait again
			c.mu.Lock()
			ch = c.notifyCh
			c.mu.Unlock()
		}
	}
}

// Update checks whether the given service has at least one ready,
// non-terminating endpoint and stores the result. Short-circuits on
// the first ready endpoint found.
func (c *ReadyEndpointsCache) Update(serviceKey string, slices []*discov1.EndpointSlice) {
	hasReady := false
	for _, slice := range slices {
		if hasAnyReadyEndpoint(slice) {
			hasReady = true
			break
		}
	}

	v, _ := c.ready.LoadOrStore(serviceKey, &atomic.Bool{})
	v.(*atomic.Bool).Store(hasReady)

	c.broadcast()
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

// hasAnyReadyEndpoint returns true if the slice contains at least one
// ready, non-terminating endpoint with at least one address.
func hasAnyReadyEndpoint(slice *discov1.EndpointSlice) bool {
	for i := range slice.Endpoints {
		ep := &slice.Endpoints[i]

		isReady := ep.Conditions.Ready == nil || *ep.Conditions.Ready
		isTerminating := ep.Conditions.Terminating != nil && *ep.Conditions.Terminating

		if isReady && !isTerminating && len(ep.Addresses) > 0 {
			return true
		}
	}
	return false
}
