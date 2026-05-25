package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// --- WaitForReady tests ---

func TestWaitForReady_AlreadyReady(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	cache.Update(key, []*discov1.EndpointSlice{
		newReadySlice("testns", "testsvc", "1.2.3.4"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.NoError(err)
	r.False(isColdStart, "should not be a cold start when already ready")
}

func TestWaitForReady_TimesOut(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.Error(err)
	r.False(isColdStart)
	r.ErrorIs(err, context.DeadlineExceeded)
	r.Contains(err.Error(), key, "error should mention the service key")
}

func TestWaitForReady_ColdStart(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cache.Update(key, []*discov1.EndpointSlice{
			newReadySlice("testns", "testsvc", "1.2.3.4"),
		})
	}()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.NoError(err)
	r.True(isColdStart, "should be a cold start when we had to wait")
}

func TestWaitForReady_IgnoresUnrelatedBroadcast(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"
	const otherKey = "testns/othersvc"

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(5 * time.Millisecond)
		cache.Update(otherKey, []*discov1.EndpointSlice{
			newReadySlice("testns", "othersvc", "5.6.7.8"),
		})
		time.Sleep(5 * time.Millisecond)
		cache.Update(key, []*discov1.EndpointSlice{
			newReadySlice("testns", "testsvc", "1.2.3.4"),
		})
	}()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.NoError(err)
	r.True(isColdStart)
}

func TestWaitForReady_ContextCancelled(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.Error(err)
	r.False(isColdStart)
	r.ErrorIs(err, context.Canceled)
}

// TestWaitForReady_ReturnsPodHost verifies that WaitForReady returns a non-empty
// podHost in "ip:port" format when the service has a ready endpoint with a port
// configured in its EndpointSlice.
func TestWaitForReady_ReturnsPodHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			Ports: []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, podHost, err := c.WaitForReady(ctx, key, "")
	r.NoError(err)
	r.Equal("1.2.3.4:8080", podHost)
}

// TestWaitForReady_NamedPortSelectsCorrectHost verifies that when an EndpointSlice
// exposes multiple named ports ("http" on 8080, "grpc" on 9090), WaitForReady
// returns the ip:port pair that matches the requested portName and never returns
// a port from a different named port.
func TestWaitForReady_NamedPortSelectsCorrectHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			Ports: []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
		{
			Ports: []discov1.EndpointPort{{Name: ptr.To("grpc"), Port: ptr.To(int32(9090))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, httpHost, err := c.WaitForReady(ctx, key, "http")
	r.NoError(err)
	r.Equal("1.2.3.4:8080", httpHost)

	_, grpcHost, err := c.WaitForReady(ctx, key, "grpc")
	r.NoError(err)
	r.Equal("1.2.3.4:9090", grpcHost)
}

// TestWaitForReady_BlocksOnNonReadyUpdates verifies that WaitForReady keeps blocking
// when the cache is updated with non-ready endpoints and only unblocks once a ready
// endpoint is published. Intermediate non-ready updates must not trigger a broadcast
// because swapState only fires one when readiness transitions from false to true.
func TestWaitForReady_BlocksOnNonReadyUpdates(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	type result struct {
		isColdStart bool
		podHost     string
		err         error
	}
	done := make(chan result, 1)
	go func() {
		isColdStart, podHost, err := c.WaitForReady(ctx, key, "")
		done <- result{isColdStart, podHost, err}
	}()

	// Non-ready updates must not unblock WaitForReady: broadcast only fires on
	// readiness transitions, so the goroutine should keep waiting.
	for range 3 {
		time.Sleep(5 * time.Millisecond)
		c.Update(key, []*discov1.EndpointSlice{
			{
				Ports: []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
				Endpoints: []discov1.Endpoint{
					{
						Addresses:  []string{"1.2.3.4"},
						Conditions: discov1.EndpointConditions{Ready: ptr.To(false)},
					},
				},
			},
		})
		select {
		case <-done:
			t.Fatal("WaitForReady returned early on a non-ready update")
		default:
		}
	}

	// Ready update: WaitForReady must now unblock as a cold start.
	c.Update(key, []*discov1.EndpointSlice{
		{
			Ports: []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
	})

	select {
	case res := <-done:
		r.NoError(res.err)
		r.True(res.isColdStart)
		r.Equal("1.2.3.4:8080", res.podHost)
	case <-ctx.Done():
		t.Fatal("WaitForReady did not unblock after ready update")
	}
}

// TestWaitForReady_UnknownPortNameReturnsEmptyHost verifies that WaitForReady returns
// an empty podHost when the requested portName has no matching candidates in the
// EndpointSlice. An empty podHost signals to the caller that direct-pod routing is
// not possible for this request and it should fall back to cluster-IP routing.
func TestWaitForReady_UnknownPortNameReturnsEmptyHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			Ports: []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, podHost, err := c.WaitForReady(ctx, key, "grpc")
	r.NoError(err)
	r.Empty(podHost, "unknown portName should return empty podHost")
}

// --- collectServiceState tests ---

func TestCollectServiceState_ReadyEndpoints(t *testing.T) {
	r := require.New(t)
	port := int32(8080)
	slice := &discov1.EndpointSlice{
		Ports: []discov1.EndpointPort{{Port: &port}},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
			{Addresses: []string{"5.6.7.8"}},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	got := make([]string, 0, len(s.candidates[""]))
	for _, ep := range s.candidates[""] {
		got = append(got, ep.ip)
		r.Equal(int32(8080), ep.port)
	}
	r.ElementsMatch([]string{"1.2.3.4", "5.6.7.8"}, got)
}

func TestCollectServiceState_NotReadyEndpoints(t *testing.T) {
	r := require.New(t)
	notReady := false
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: &notReady},
			},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.False(s.ready)
}

// TestCollectServiceState_NilReadyTreatedAsReady verifies that an endpoint with a nil
// Ready condition is counted as ready. The Kubernetes API spec treats nil as "ready"
// and sets it explicitly to false only for terminating pods; services configured with
// publishNotReadyAddresses always have nil here.
func TestCollectServiceState_NilReadyTreatedAsReady(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready, "nil Ready should be treated as ready per K8s API spec")
}

func TestCollectServiceState_ExtractsPort(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Ports: []discov1.EndpointPort{
			{Port: ptr.To(int32(8080))},
		},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.Len(s.candidates[""], 1)
	r.Equal(int32(8080), s.candidates[""][0].port)
}

// TestCollectServiceState_DeduplicatesIPs verifies that the same (ip, port) pair
// appearing in multiple EndpointSlices is added to candidates only once. Kubernetes
// may split endpoints across several slices for the same service, so overlap is
// possible and must not inflate the candidate pool with duplicates.
func TestCollectServiceState_DeduplicatesIPs(t *testing.T) {
	r := require.New(t)
	port := int32(8080)
	// Two slices with overlapping IPs — the duplicate (ip, port) pair must appear only once.
	s := collectServiceState([]*discov1.EndpointSlice{
		{
			Ports:     []discov1.EndpointPort{{Port: &port}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			Ports:     []discov1.EndpointPort{{Port: &port}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4", "5.6.7.8"}}},
		},
	})
	r.True(s.ready)
	r.Len(s.candidates[""], 2)
}

// TestCollectServiceState_HeterogeneousPortsSamePortName verifies that two slices
// sharing the same portName but different port numbers are both retained as separate
// candidates. This is the rolling-deploy scenario: old pods expose containerPort 8080
// and new pods expose 9090 under the same named port. Each (ip, port) pair must be
// kept intact so that pickHost never returns a mismatched host.
func TestCollectServiceState_HeterogeneousPortsSamePortName(t *testing.T) {
	r := require.New(t)
	httpName := "http"
	port8080 := int32(8080)
	port9090 := int32(9090)

	// Two slices carry the same portName "http" but different port numbers —
	// the rolling-deploy scenario the reviewer identified.
	s := collectServiceState([]*discov1.EndpointSlice{
		{
			Ports:     []discov1.EndpointPort{{Name: &httpName, Port: &port8080}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			Ports:     []discov1.EndpointPort{{Name: &httpName, Port: &port9090}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"10.0.0.1"}}},
		},
	})

	r.True(s.ready)
	r.Len(s.candidates["http"], 2, "both (ip,port) pairs must be kept")

	// Build a map from IP → port so we can verify no mismatching occurs.
	ipToPort := make(map[string]int32)
	for _, ep := range s.candidates["http"] {
		ipToPort[ep.ip] = ep.port
	}
	r.Equal(int32(8080), ipToPort["1.2.3.4"])
	r.Equal(int32(9090), ipToPort["10.0.0.1"])

	// Verify WaitForReady never returns a mismatched host (ip:port) pair.
	c := NewReadyEndpointsCache(logr.Discard())
	c.Update("testns/testsvc", []*discov1.EndpointSlice{
		{
			Ports:     []discov1.EndpointPort{{Name: &httpName, Port: &port8080}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			Ports:     []discov1.EndpointPort{{Name: &httpName, Port: &port9090}},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"10.0.0.1"}}},
		},
	})
	for range 100 {
		_, podHost, err := c.WaitForReady(context.Background(), "testns/testsvc", "http")
		r.NoError(err)
		// podHost must be one of the two valid ip:port pairs — never a mismatch.
		r.Contains([]string{"1.2.3.4:8080", "10.0.0.1:9090"}, podHost,
			"host must be a consistent ip:port pair from the same slice")
	}
}

// TestCollectServiceState_SkipsNilPort verifies that a port entry whose Port field is
// nil is ignored. Kubernetes sets Port to nil for ports that are not yet assigned;
// including such an entry would produce an invalid (ip, 0) candidate.
func TestCollectServiceState_SkipsNilPort(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Ports: []discov1.EndpointPort{
			{Port: nil},
			{Port: ptr.To(int32(8080))},
		},
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	r.Len(s.candidates[""], 1)
	r.Equal(int32(8080), s.candidates[""][0].port, "nil port entry must not produce a candidate")
}

// TestCollectServiceState_SkipsEndpointWithNoAddresses verifies that an endpoint
// with an empty Addresses slice is not counted as ready and produces no candidates.
// This guards against malformed EndpointSlice objects that pass the Ready check but
// carry no routable IP.
func TestCollectServiceState_SkipsEndpointWithNoAddresses(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Ports: []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
			},
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	r.Len(s.candidates[""], 1, "empty-address endpoint must not produce a candidate")
	r.Equal("1.2.3.4", s.candidates[""][0].ip)
}

// --- Update tests ---

func TestUpdate_ClearsStateOnEmptySlices(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySlice("testns", "testsvc", "1.2.3.4"),
	})

	c.Update(key, nil) // clear

	r.False(c.HasReadyEndpoints(key))
}

func TestUpdateDeletesKeyWhenNoSlices(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	cache.Update(key, []*discov1.EndpointSlice{
		newReadySlice("testns", "testsvc", "1.2.3.4"),
	})

	r.True(cache.HasReadyEndpoints(key))
	_, ok := cache.states.Load(key)
	r.True(ok, "key should exist after update with slices")

	cache.Update(key, nil)

	r.False(cache.HasReadyEndpoints(key))
	_, ok = cache.states.Load(key)
	r.False(ok, "key should be removed when service has no slices")
}

func TestUpdateRetainsKeyForNonReadySlices(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	notReady := false
	cache.Update(key, []*discov1.EndpointSlice{
		{
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: &notReady},
				},
			},
		},
	})

	r.False(cache.HasReadyEndpoints(key))
	_, ok := cache.states.Load(key)
	r.True(ok, "key should remain when slices exist but none are ready")
}

// --- endpointSliceFromDeleteObj tests ---

func TestEndpointSliceFromDeleteObj_DirectObject(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-slice",
			Namespace: "testns",
		},
	}

	got, err := endpointSliceFromDeleteObj(slice)
	r.NoError(err)
	r.Equal(slice, got)
}

func TestEndpointSliceFromDeleteObj_TombstoneValue(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-slice",
			Namespace: "testns",
		},
	}

	got, err := endpointSliceFromDeleteObj(cache.DeletedFinalStateUnknown{Obj: slice})
	r.NoError(err)
	r.Equal(slice, got)
}

func TestEndpointSliceFromDeleteObj_InvalidTombstonePayload(t *testing.T) {
	r := require.New(t)

	_, err := endpointSliceFromDeleteObj(cache.DeletedFinalStateUnknown{Obj: "not-an-endpointslice"})
	r.Error(err)
}

// TestEndpointSliceFromDeleteObj_UnknownType verifies that an object that is neither
// an *EndpointSlice nor a DeletedFinalStateUnknown tombstone returns an error. This
// guards the default branch of the type switch against unexpected informer payloads.
func TestEndpointSliceFromDeleteObj_UnknownType(t *testing.T) {
	r := require.New(t)

	_, err := endpointSliceFromDeleteObj("unexpected-string-type")
	r.Error(err)
}

// --- helpers ---

func newReadySlice(namespace, service string, addresses ...string) *discov1.EndpointSlice {
	endpoints := make([]discov1.Endpoint, 0, len(addresses))
	for _, addr := range addresses {
		endpoints = append(endpoints, discov1.Endpoint{
			Addresses: []string{addr},
		})
	}

	return &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service + "-slice",
			Namespace: namespace,
			Labels: map[string]string{
				discov1.LabelServiceName: service,
			},
		},
		Endpoints: endpoints,
	}
}
