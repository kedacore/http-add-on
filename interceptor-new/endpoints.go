package main

import (
	"sync"
	"sync/atomic"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var endpointsLog = ctrl.Log.WithName("endpoints")

// EndpointsCache maintains a derived map of service -> ready-endpoint-count
// for O(1) hot-path lookups, plus a broadcast mechanism so the cold-start
// wait function can block until a service becomes ready.
//
// Hot path (warm backend): single atomic load — no locks, no allocations.
// Cold path (scale-from-zero): subscribe to broadcast channel, wait for notification.
type EndpointsCache struct {
	// "namespace/service" -> *atomic.Int64 (ready endpoint count)
	readyCounts sync.Map

	// Broadcast mechanism: the channel is closed on any change,
	// then replaced with a fresh one. Waiters select on the channel.
	mu       sync.Mutex
	notifyCh chan struct{}
}

// NewEndpointsCache creates a new empty endpoints cache.
func NewEndpointsCache() *EndpointsCache {
	return &EndpointsCache{
		notifyCh: make(chan struct{}),
	}
}

// HasReadyEndpoints returns true if the service has at least one ready endpoint.
// This is the fast hot-path check (one atomic load).
func (c *EndpointsCache) HasReadyEndpoints(serviceKey string) bool {
	if v, ok := c.readyCounts.Load(serviceKey); ok {
		return v.(*atomic.Int64).Load() > 0
	}
	return false
}

// WaitForReady waits until the service has at least one ready endpoint.
// Returns:
//   - (false, nil)  — warm backend, already ready (fast path)
//   - (true, nil)   — cold start, but backend became ready
//   - (false, error) — timeout waiting for ready endpoints
func (c *EndpointsCache) WaitForReady(serviceKey string, timeout time.Duration) (isColdStart bool, err error) {
	// Fast path: already ready
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

	endpointsLog.V(1).Info("cold-start: waiting for ready endpoints", "key", serviceKey)

	// Slow path: wait for notification or timeout
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ch:
			// Something changed — check if our service is ready
			if c.HasReadyEndpoints(serviceKey) {
				endpointsLog.Info("cold-start: endpoints became ready", "key", serviceKey)
				return true, nil
			}
			// Not our service — get the new channel and wait again
			c.mu.Lock()
			ch = c.notifyCh
			c.mu.Unlock()

		case <-deadline.C:
			endpointsLog.Info("cold-start: timed out waiting for ready endpoints",
				"key", serviceKey,
				"timeout", timeout,
			)
			return false, errEndpointTimeout
		}
	}
}

// UpdateService recounts ready endpoints for the given service from the
// provided EndpointSlice list. Called by the EndpointSlice informer handler.
func (c *EndpointsCache) UpdateService(namespace, service string, slices []*discoveryv1.EndpointSlice) {
	key := namespace + "/" + service

	var total int64
	for _, slice := range slices {
		total += countReadyEndpoints(slice)
	}

	// Store the count
	v, _ := c.readyCounts.LoadOrStore(key, &atomic.Int64{})
	v.(*atomic.Int64).Store(total)

	// Broadcast the change (wake all waiters)
	c.broadcast()
}

// broadcast wakes all waiting goroutines by closing the current channel
// and replacing it with a new one.
func (c *EndpointsCache) broadcast() {
	c.mu.Lock()
	old := c.notifyCh
	c.notifyCh = make(chan struct{})
	c.mu.Unlock()
	close(old)
}

// countReadyEndpoints counts the number of ready, non-terminating endpoint
// addresses in a single EndpointSlice.
func countReadyEndpoints(slice *discoveryv1.EndpointSlice) int64 {
	var total int64
	for i := range slice.Endpoints {
		ep := &slice.Endpoints[i]
		cond := ep.Conditions

		isReady := true
		if cond.Ready != nil {
			isReady = *cond.Ready
		}

		isTerminating := false
		if cond.Terminating != nil {
			isTerminating = *cond.Terminating
		}

		if isReady && !isTerminating {
			total += int64(len(ep.Addresses))
		}
	}
	return total
}

// extractServiceFromSlice returns (namespace, service-name) from an EndpointSlice's labels.
func extractServiceFromSlice(slice *discoveryv1.EndpointSlice) (string, string, bool) {
	ns := slice.Namespace
	if ns == "" {
		ns = defaultNamespace
	}
	svcName, ok := slice.Labels["kubernetes.io/service-name"]
	if !ok || svcName == "" {
		return "", "", false
	}
	return ns, svcName, true
}

// sentinel error
type endpointTimeoutError struct{}

func (endpointTimeoutError) Error() string { return "timed out waiting for ready endpoints" }

var errEndpointTimeout error = endpointTimeoutError{}
