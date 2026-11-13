package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
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

	isColdStart, err := cache.WaitForReady(ctx, key)
	r.NoError(err)
	r.False(isColdStart, "should not be a cold start when already ready")
}

func TestWaitForReady_TimesOut(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	isColdStart, err := cache.WaitForReady(ctx, key)
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
		time.Sleep(100 * time.Millisecond)
		cache.Update(key, []*discov1.EndpointSlice{
			newReadySlice("testns", "testsvc", "1.2.3.4"),
		})
	}()

	isColdStart, err := cache.WaitForReady(ctx, key)
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
		time.Sleep(50 * time.Millisecond)
		cache.Update(otherKey, []*discov1.EndpointSlice{
			newReadySlice("testns", "othersvc", "5.6.7.8"),
		})
		time.Sleep(50 * time.Millisecond)
		cache.Update(key, []*discov1.EndpointSlice{
			newReadySlice("testns", "testsvc", "1.2.3.4"),
		})
	}()

	isColdStart, err := cache.WaitForReady(ctx, key)
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
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	isColdStart, err := cache.WaitForReady(ctx, key)
	r.Error(err)
	r.False(isColdStart)
	r.ErrorIs(err, context.Canceled)
}

func TestWaitForReady_BlocksOnNonReadyUpdates(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	type result struct {
		isColdStart bool
		err         error
	}
	done := make(chan result, 1)
	go func() {
		isColdStart, err := c.WaitForReady(ctx, key)
		done <- result{isColdStart, err}
	}()

	for range 3 {
		time.Sleep(5 * time.Millisecond)
		c.Update(key, []*discov1.EndpointSlice{
			{
				AddressType: discov1.AddressTypeIPv4,
				Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
				Endpoints: []discov1.Endpoint{
					{
						Addresses:  []string{"1.2.3.4"},
						Conditions: discov1.EndpointConditions{Ready: new(false)},
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

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: new(true)},
				},
			},
		},
	})

	select {
	case res := <-done:
		r.NoError(res.err)
		r.True(res.isColdStart)
	case <-ctx.Done():
		t.Fatal("WaitForReady did not unblock after ready update")
	}
}

// --- PickEndpoint tests ---

func TestPickEndpoint_ReturnsHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: new(true)},
				},
			},
		},
	})

	ep, ok := c.PickEndpoint(key, "", "")
	r.True(ok)
	r.Equal("1.2.3.4:8080", ep.Host)
}

func TestPickEndpoint_NamedPortSelectsCorrectHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: new("http"), Port: new(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: new(true)},
				},
			},
		},
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: new("grpc"), Port: new(int32(9090))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: new(true)},
				},
			},
		},
	})

	httpEp, ok := c.PickEndpoint(key, "http", "")
	r.True(ok)
	r.Equal("1.2.3.4:8080", httpEp.Host)

	grpcEp, ok := c.PickEndpoint(key, "grpc", "")
	r.True(ok)
	r.Equal("1.2.3.4:9090", grpcEp.Host)
}

func TestPickEndpoint_UnknownPortNameReturnsFalse(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: new("http"), Port: new(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: new(true)},
				},
			},
		},
	})

	_, ok := c.PickEndpoint(key, "grpc", "")
	r.False(ok, "unknown portName should not yield an endpoint")
}

func TestPickEndpoint_UnknownServiceReturnsFalse(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())

	_, ok := c.PickEndpoint("testns/unknown", "", "")
	r.False(ok)
}

func TestPickEndpoint_PrefersEndpointWithID(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySliceWithPods("testns", "testsvc", 8080, map[string]types.UID{
			"1.2.3.4": "uid-a",
			"5.6.7.8": "uid-b",
			"9.9.9.9": "uid-c",
		}),
	})

	preferred := podID("uid-b")
	for range 50 {
		ep, ok := c.PickEndpoint(key, "", preferred)
		r.True(ok)
		r.Equal("5.6.7.8:8080", ep.Host)
		r.Equal(preferred, ep.ID)
	}
}

func TestPickEndpoint_UnknownPreferredIDPicksAnyEndpoint(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySliceWithPods("testns", "testsvc", 8080, map[string]types.UID{
			"1.2.3.4": "uid-a",
			"5.6.7.8": "uid-b",
		}),
	})

	seen := make(map[string]string)
	for range 100 {
		ep, ok := c.PickEndpoint(key, "", "unknown")
		r.True(ok)
		seen[ep.Host] = ep.ID
	}
	r.Equal(map[string]string{
		"1.2.3.4:8080": podID("uid-a"),
		"5.6.7.8:8080": podID("uid-b"),
	}, seen, "unknown preferred ID must fall back to random selection across all pods")
}

// --- collectServiceState tests ---

func TestCollectServiceState_ReadyEndpoints(t *testing.T) {
	r := require.New(t)
	port := int32(8080)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
		Ports:       []discov1.EndpointPort{{Port: &port}},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
			{Addresses: []string{"5.6.7.8"}},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	got := make([]string, 0)
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
		AddressType: discov1.AddressTypeIPv4,
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

func TestCollectServiceState_NilReadyTreatedAsReady(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
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
		AddressType: discov1.AddressTypeIPv4,
		Ports: []discov1.EndpointPort{
			{Port: new(int32(8080))},
		},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.Len(s.candidates[""], 1)
	r.Equal(int32(8080), s.candidates[""][0].port)
}

// TestCollectServiceState_DeduplicatesPods verifies that the same pod (same
// address list + port) appearing in transiently-overlapping slices is counted
// once, so it is not weighted multiple times in random selection.
func TestCollectServiceState_DeduplicatesPods(t *testing.T) {
	r := require.New(t)
	port := int32(8080)
	s := collectServiceState([]*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: &port}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: &port}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
	})
	r.True(s.ready)
	r.Len(s.candidates[""], 1, "the same pod across overlapping slices must be deduped")
}

func TestCollectServiceState_HeterogeneousPortsSamePortName(t *testing.T) {
	r := require.New(t)
	httpName := "http"
	port8080 := int32(8080)
	port9090 := int32(9090)

	s := collectServiceState([]*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: &httpName, Port: &port8080}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: &httpName, Port: &port9090}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"10.0.0.1"}}},
		},
	})

	r.True(s.ready)
	r.Len(s.candidates["http"], 2, "both (ip,port) pairs must be kept")

	ipToPort := make(map[string]int32)
	for _, ep := range s.candidates["http"] {
		ipToPort[ep.ip] = ep.port
	}
	r.Equal(int32(8080), ipToPort["1.2.3.4"])
	r.Equal(int32(9090), ipToPort["10.0.0.1"])

	c := NewReadyEndpointsCache(logr.Discard())
	c.Update("testns/testsvc", []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: &httpName, Port: &port8080}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: &httpName, Port: &port9090}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"10.0.0.1"}}},
		},
	})
	for range 100 {
		ep, ok := c.PickEndpoint("testns/testsvc", "http", "")
		r.True(ok)
		r.Contains([]string{"1.2.3.4:8080", "10.0.0.1:9090"}, ep.Host,
			"host must be a consistent ip:port pair from the same slice")
	}
}

func TestCollectServiceState_SkipsNilPort(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
		Ports: []discov1.EndpointPort{
			{Port: nil},
			{Port: new(int32(8080))},
		},
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: new(true)},
			},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	r.Len(s.candidates[""], 1)
	r.Equal(int32(8080), s.candidates[""][0].port, "nil port entry must not produce a candidate")
}

func TestCollectServiceState_SkipsEndpointWithNoAddresses(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
		Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{},
				Conditions: discov1.EndpointConditions{Ready: new(true)},
			},
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: new(true)},
			},
		},
	}
	s := collectServiceState([]*discov1.EndpointSlice{slice})
	r.True(s.ready)
	r.Len(s.candidates[""], 1, "empty-address endpoint must not produce a candidate")
	r.Equal("1.2.3.4", s.candidates[""][0].ip)
}

func TestCollectServiceState_EndpointID(t *testing.T) {
	tests := map[string]struct {
		targetRef *corev1.ObjectReference
		address   string
		wantID    string
	}{
		"uses targetRef UID": {
			targetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-a", UID: "uid-a"},
			address:   "1.2.3.4",
			wantID:    podID("uid-a"),
		},
		"falls back to address without targetRef": {
			address: "1.2.3.4",
			wantID:  podID("1.2.3.4"),
		},
		"falls back to address when targetRef has no UID": {
			targetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-a"},
			address:   "1.2.3.4",
			wantID:    podID("1.2.3.4"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			s := collectServiceState([]*discov1.EndpointSlice{{
				AddressType: discov1.AddressTypeIPv4,
				Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
				Endpoints: []discov1.Endpoint{
					{Addresses: []string{tt.address}, TargetRef: tt.targetRef},
				},
			}})
			r.Len(s.candidates[""], 1)
			r.Equal(tt.wantID, s.candidates[""][0].id)
			r.Len(tt.wantID, 16)
		})
	}
}

func TestCollectServiceState_EndpointsWithoutUIDGetDistinctIDs(t *testing.T) {
	r := require.New(t)
	s := collectServiceState([]*discov1.EndpointSlice{{
		AddressType: discov1.AddressTypeIPv4,
		Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-a"}},
			{Addresses: []string{"5.6.7.8"}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-b"}},
		},
	}})
	r.Len(s.candidates[""], 2)
	r.NotEqual(s.candidates[""][0].id, s.candidates[""][1].id)
}

// TestCollectServiceState_DualStackSharesID verifies that the IPv4 and IPv6
// addresses of one pod resolve to the same identity.
func TestCollectServiceState_DualStackSharesID(t *testing.T) {
	r := require.New(t)
	ref := &corev1.ObjectReference{Kind: "Pod", Name: "pod-a", UID: "uid-a"}
	s := collectServiceState([]*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}, TargetRef: ref}},
		},
		{
			AddressType: discov1.AddressTypeIPv6,
			Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
			Endpoints:   []discov1.Endpoint{{Addresses: []string{"fd00::1"}, TargetRef: ref}},
		},
	})
	r.Len(s.candidates[""], 2, "both address families must remain candidates")
	for _, ep := range s.candidates[""] {
		r.Equal(podID("uid-a"), ep.id)
	}
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
			AddressType: discov1.AddressTypeIPv4,
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

func TestEndpointSliceFromDeleteObj_UnknownType(t *testing.T) {
	r := require.New(t)

	_, err := endpointSliceFromDeleteObj("unexpected-string-type")
	r.Error(err)
}

// --- helpers ---

func newReadySliceWithPods(namespace, service string, port int32, pods map[string]types.UID) *discov1.EndpointSlice {
	slice := newReadySlice(namespace, service)
	slice.Ports = []discov1.EndpointPort{{Port: &port}}
	for addr, uid := range pods {
		slice.Endpoints = append(slice.Endpoints, discov1.Endpoint{
			Addresses: []string{addr},
			TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "pod-" + string(uid), UID: uid},
		})
	}
	return slice
}

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
		AddressType: discov1.AddressTypeIPv4,
		Endpoints:   endpoints,
	}
}
