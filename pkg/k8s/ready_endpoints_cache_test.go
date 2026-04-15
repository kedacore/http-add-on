package k8s

import (
	"context"
	"sort"
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

// --- Update / key retention tests ---

func TestUpdateDeletesKeyWhenNoSlices(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	cache.Update(key, []*discov1.EndpointSlice{
		newReadySlice("testns", "testsvc", "1.2.3.4"),
	})

	r.True(cache.HasReadyEndpoints(key))
	_, ok := cache.endpoints.Load(key)
	r.True(ok, "key should exist after update with slices")

	cache.Update(key, nil)

	r.False(cache.HasReadyEndpoints(key))
	_, ok = cache.endpoints.Load(key)
	r.False(ok, "key should be removed when service has no slices")
}

func TestUpdateRetainsKeyForNonReadySlices(t *testing.T) {
	r := require.New(t)
	cache := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	cache.Update(key, []*discov1.EndpointSlice{
		newReadySlice("testns", "testsvc"),
	})

	r.False(cache.HasReadyEndpoints(key))
	_, ok := cache.endpoints.Load(key)
	r.True(ok, "key should remain when slices exist but none are ready")
}

// --- hasAnyReadyEndpoint tests ---

func TestHasAnyReadyEndpoint_ReadyWithAddress(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"1.2.3.4"}},
		},
	}
	r.True(hasAnyReadyEndpoint(slice))
}

func TestHasAnyReadyEndpoint_ExplicitReady(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(true)},
			},
		},
	}
	r.True(hasAnyReadyEndpoint(slice))
}

func TestHasAnyReadyEndpoint_NotReady(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(false)},
			},
		},
	}
	r.False(hasAnyReadyEndpoint(slice))
}

func TestHasAnyReadyEndpoint_NoAddresses(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{}},
		},
	}
	r.False(hasAnyReadyEndpoint(slice))
}

func TestHasAnyReadyEndpoint_EmptySlice(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{}
	r.False(hasAnyReadyEndpoint(slice))
}

func TestHasAnyReadyEndpoint_MixedEndpoints(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{Ready: ptr.To(false)},
			},
			{
				Addresses: []string{"5.6.7.8"},
			},
		},
	}
	r.True(hasAnyReadyEndpoint(slice), "should find the second endpoint with nil Ready (defaults to ready)")
}

func TestHasAnyReadyEndpoint_NilReadyIsReady(t *testing.T) {
	r := require.New(t)
	slice := &discov1.EndpointSlice{
		Endpoints: []discov1.Endpoint{
			{
				Addresses:  []string{"1.2.3.4"},
				Conditions: discov1.EndpointConditions{},
			},
		},
	}
	r.True(hasAnyReadyEndpoint(slice), "nil Ready should be treated as ready per K8s API spec")
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

// --- GetEndpoints tests ---

func TestGetEndpoints_Unknown(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	r.Nil(c.GetEndpoints("testns/unknown"))
}

func TestGetEndpoints_ReturnsSnapshot(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySliceWithPorts("testns", "testsvc",
			[]discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			"10.0.0.1", "10.0.0.2"),
	})

	se := c.GetEndpoints(key)
	r.NotNil(se)
	sort.Strings(se.Addresses)
	r.Equal([]string{"10.0.0.1", "10.0.0.2"}, se.Addresses)
	r.Equal(map[string]int32{"http": 8080}, se.Ports)
}

func TestGetEndpoints_DeduplicatesAddresses(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySliceWithPorts("testns", "testsvc",
			[]discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			"10.0.0.1", "10.0.0.2"),
		newReadySliceWithPorts("testns", "testsvc",
			[]discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
			"10.0.0.2", "10.0.0.3"),
	})

	se := c.GetEndpoints(key)
	r.NotNil(se)
	sort.Strings(se.Addresses)
	r.Equal([]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, se.Addresses)
}

func TestGetEndpoints_ExcludesNotReady(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	slice := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testsvc-slice", Namespace: "testns",
			Labels: map[string]string{discov1.LabelServiceName: "testsvc"},
		},
		Ports: []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discov1.EndpointConditions{Ready: ptr.To(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discov1.EndpointConditions{Ready: ptr.To(false)}},
		},
	}
	c.Update(key, []*discov1.EndpointSlice{slice})

	se := c.GetEndpoints(key)
	r.NotNil(se)
	r.Equal([]string{"10.0.0.1"}, se.Addresses)
}

func TestGetEndpoints_UnnamedPort(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())
	const key = "testns/testsvc"

	c.Update(key, []*discov1.EndpointSlice{
		newReadySliceWithPorts("testns", "testsvc",
			[]discov1.EndpointPort{{Port: ptr.To(int32(9090))}},
			"10.0.0.1"),
	})

	se := c.GetEndpoints(key)
	r.NotNil(se)
	r.Equal(map[string]int32{"": 9090}, se.Ports)
}

// --- buildServiceEndpoints tests ---

func TestBuildServiceEndpoints_MultiplePorts(t *testing.T) {
	r := require.New(t)
	slices := []*discov1.EndpointSlice{{
		Ports: []discov1.EndpointPort{
			{Name: ptr.To("http"), Port: ptr.To(int32(8080))},
			{Name: ptr.To("grpc"), Port: ptr.To(int32(9090))},
		},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"10.0.0.1"}},
		},
	}}

	se := buildServiceEndpoints(slices)
	r.Equal(map[string]int32{"http": 8080, "grpc": 9090}, se.Ports)
	r.Equal([]string{"10.0.0.1"}, se.Addresses)
}

func TestBuildServiceEndpoints_EmptySlices(t *testing.T) {
	r := require.New(t)
	se := buildServiceEndpoints(nil)
	r.Empty(se.Addresses)
	r.Empty(se.Ports)
}

func TestBuildServiceEndpoints_OnlyNotReadyEndpoints(t *testing.T) {
	r := require.New(t)
	slices := []*discov1.EndpointSlice{{
		Ports: []discov1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(8080))}},
		Endpoints: []discov1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discov1.EndpointConditions{Ready: ptr.To(false)}},
		},
	}}

	se := buildServiceEndpoints(slices)
	r.Empty(se.Addresses)
	r.Equal(map[string]int32{"http": 8080}, se.Ports)
}

// --- helpers ---

func newReadySlice(namespace, service string, addresses ...string) *discov1.EndpointSlice {
	return newReadySliceWithPorts(namespace, service, nil, addresses...)
}

func newReadySliceWithPorts(namespace, service string, ports []discov1.EndpointPort, addresses ...string) *discov1.EndpointSlice {
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
		Ports:     ports,
		Endpoints: endpoints,
	}
}
