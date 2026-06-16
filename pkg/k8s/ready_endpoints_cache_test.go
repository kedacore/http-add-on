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
		time.Sleep(100 * time.Millisecond)
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
		time.Sleep(50 * time.Millisecond)
		cache.Update(otherKey, []*discov1.EndpointSlice{
			newReadySlice("testns", "othersvc", "5.6.7.8"),
		})
		time.Sleep(50 * time.Millisecond)
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
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	isColdStart, _, err := cache.WaitForReady(ctx, key, "")
	r.Error(err)
	r.False(isColdStart)
	r.ErrorIs(err, context.Canceled)
}

func TestWaitForReady_ReturnsPodHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
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

func TestWaitForReady_NamedPortSelectsCorrectHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			Endpoints: []discov1.Endpoint{
				{
					Addresses:  []string{"1.2.3.4"},
					Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
				},
			},
		},
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: ptr.To("grpc"), Port: ptr.To(int32(9090))}},
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

	for range 3 {
		time.Sleep(5 * time.Millisecond)
		c.Update(key, []*discov1.EndpointSlice{
			{
				AddressType: discov1.AddressTypeIPv4,
				Ports:       []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
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

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
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

func TestWaitForReady_UnknownPortNameReturnsEmptyHost(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		{
			AddressType: discov1.AddressTypeIPv4,
			Ports:       []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
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
		_, podHost, err := c.WaitForReady(context.Background(), "testns/testsvc", "http")
		r.NoError(err)
		r.Contains([]string{"1.2.3.4:8080", "10.0.0.1:9090"}, podHost,
			"host must be a consistent ip:port pair from the same slice")
	}
}

func TestCollectServiceState_SkipsNilPort(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
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

func TestCollectServiceState_SkipsEndpointWithNoAddresses(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		AddressType: discov1.AddressTypeIPv4,
		Ports:       []discov1.EndpointPort{{Port: ptr.To(int32(8080))}},
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
