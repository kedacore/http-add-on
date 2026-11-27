package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	discov1 "k8s.io/api/discovery/v1"
)

func TestParseLoadBalancing(t *testing.T) {
	r := require.New(t)

	for in, want := range map[string]LoadBalancing{
		"":              LoadBalancingRandom,
		"random":        LoadBalancingRandom,
		"least-request": LoadBalancingLeastRequest,
	} {
		got, err := ParseLoadBalancing(in)
		r.NoError(err, in)
		r.Equal(want, got, in)
	}

	_, err := ParseLoadBalancing("round-robin")
	r.ErrorContains(err, `invalid load balancing mode "round-robin"`)
}

func TestInFlightRequests_StartAndDone(t *testing.T) {
	r := require.New(t)
	f := newInFlightRequests()

	done1 := f.start("1.2.3.4:8080")
	done2 := f.start("1.2.3.4:8080")
	r.Equal(2, f.count("1.2.3.4:8080"))
	r.Equal(0, f.count("5.6.7.8:8080"))

	done1()
	done1() // calling done twice only counts once
	r.Equal(1, f.count("1.2.3.4:8080"))

	done2()
	r.Equal(0, f.count("1.2.3.4:8080"))
	r.Empty(f.counts, "hosts without in-flight requests are removed")
}

func newLeastRequestCache(t *testing.T, key string, addresses ...string) *ReadyEndpointsCache {
	t.Helper()
	c := NewReadyEndpointsCache(logr.Discard())
	c.SetLoadBalancing(LoadBalancingLeastRequest)
	endpoints := make([]discov1.Endpoint, 0, len(addresses))
	for _, addr := range addresses {
		endpoints = append(endpoints, discov1.Endpoint{Addresses: []string{addr}})
	}
	c.Update(key, []*discov1.EndpointSlice{{
		AddressType: discov1.AddressTypeIPv4,
		Ports:       []discov1.EndpointPort{{Port: new(int32(8080))}},
		Endpoints:   endpoints,
	}})
	return c
}

func TestWaitForReady_LeastRequestAvoidsBusyPod(t *testing.T) {
	r := require.New(t)
	const key = "testns/testsvc"
	c := newLeastRequestCache(t, key, "1.1.1.1", "2.2.2.2", "3.3.3.3")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// The two random picks are always different pods, so a pod with more
	// in-flight requests than all others is never picked.
	done := c.StartRequest("1.1.1.1:8080")
	seen := map[string]bool{}
	for range 200 {
		_, podHost, err := c.WaitForReady(ctx, key, "")
		r.NoError(err)
		r.NotEqual("1.1.1.1:8080", podHost)
		seen[podHost] = true
	}
	r.Len(seen, 2, "the idle pods are both used")

	// Once the request is done, the pod is picked again.
	done()
	seen = map[string]bool{}
	for range 200 {
		_, podHost, err := c.WaitForReady(ctx, key, "")
		r.NoError(err)
		seen[podHost] = true
	}
	r.True(seen["1.1.1.1:8080"])
}

func TestWaitForReady_LeastRequestSinglePod(t *testing.T) {
	r := require.New(t)
	const key = "testns/testsvc"
	c := newLeastRequestCache(t, key, "1.1.1.1")

	defer c.StartRequest("1.1.1.1:8080")()
	_, podHost, err := c.WaitForReady(context.Background(), key, "")
	r.NoError(err)
	r.Equal("1.1.1.1:8080", podHost)
}

func TestStartRequest_RandomDoesNotCount(t *testing.T) {
	r := require.New(t)
	c := NewReadyEndpointsCache(logr.Discard())

	c.StartRequest("1.1.1.1:8080")()
	r.Nil(c.inFlight)

	c.SetLoadBalancing(LoadBalancingLeastRequest)
	done := c.StartRequest("1.1.1.1:8080")
	r.Equal(1, c.inFlight.count("1.1.1.1:8080"))
	done()
	r.Equal(0, c.inFlight.count("1.1.1.1:8080"))
}
