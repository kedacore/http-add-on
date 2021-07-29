package main

import (
	context "context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"github.com/stretchr/testify/require"
)

func TestIsActive(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()

	hdl := newImpl(
		lggr,
		pinger,
	)
	res, err := hdl.IsActive(
		ctx,
		&externalscaler.ScaledObjectRef{},
	)
	r.NoError(err)
	r.NotNil(res)
	r.True(res.Result)
}

func TestGetMetricSpec(t *testing.T) {
	const (
		host   = "abcd"
		target = int64(200)
	)
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	hdl := newImpl(lggr, pinger)
	meta := map[string]string{
		"host":   host,
		"target": strconv.Itoa(int(target)),
	}
	ref := &externalscaler.ScaledObjectRef{
		ScalerMetadata: meta,
	}
	ret, err := hdl.GetMetricSpec(ctx, ref)
	r.NoError(err)
	r.NotNil(ret)
	r.Equal(1, len(ret.MetricSpecs))
	spec := ret.MetricSpecs[0]
	r.Equal(host, spec.MetricName)
	r.Equal(target, spec.TargetSize)
}

// GetMetrics with a ScaledObjectRef in the RPC request that has
// no 'host' field in the metadata field
func TestGetMetricsMissingHostInMetadata(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	req := &externalscaler.GetMetricsRequest{
		ScaledObjectRef: &externalscaler.ScaledObjectRef{},
	}
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	hdl := newImpl(lggr, pinger)

	// no 'host' in the ScalerObjectRef's metadata field
	res, err := hdl.GetMetrics(ctx, req)
	r.Error(err)
	r.Nil(res)
	r.Contains(
		err.Error(),
		"no 'host' field found in ScaledObject metadata",
	)
}

// 'host' field found in ScalerObjectRef.ScalerMetadata, but
// not found in the queuePinger
func TestGetMetricsMissingHostInQueue(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()

	const host = "TestGetMetricsMissingHostInQueue.com"
	meta := map[string]string{
		"host": host,
	}

	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	hdl := newImpl(lggr, pinger)

	req := &externalscaler.GetMetricsRequest{
		ScaledObjectRef: &externalscaler.ScaledObjectRef{},
	}
	req.ScaledObjectRef.ScalerMetadata = meta
	res, err := hdl.GetMetrics(ctx, req)
	r.Error(err)
	r.Contains(err.Error(), fmt.Sprintf(
		"host '%s' not found in counts", host,
	))
	r.Nil(res)
}

// GetMetrics RPC call with host found in both the incoming
// ScaledObject and in the queue counter
func TestGetMetricsHostFoundInQueueCounts(t *testing.T) {
	const (
		host        = "TestGetMetricsHostFoundInQueueCounts.com"
		ns          = "testns"
		svcName     = "testsrv"
		pendingQLen = 203
	)

	// create a request for the GetMetrics RPC call. it instructs
	// GetMetrics to return the counts for one specific host.
	// below, we do setup to ensure that we have a fake
	// interceptor, and that interceptor knows about the given host
	req := &externalscaler.GetMetricsRequest{
		ScaledObjectRef: &externalscaler.ScaledObjectRef{
			ScalerMetadata: map[string]string{
				"host": host,
			},
		},
	}

	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()

	// we need to create a new queuePinger with valid endpoints
	// to query this time, so that when counts are requested by
	// the internal queuePinger logic, there is a valid host from
	// which to request those counts
	q := queue.NewFakeCounter()
	// NOTE: don't call .Resize here or you'll have to make sure
	// to receive on q.ResizedCh
	q.RetMap[host] = pendingQLen

	// create a fake interceptor
	fakeSrv, fakeSrvURL, endpoints, err := startFakeQueueEndpointServer(
		ns,
		svcName,
		q,
		1,
	)
	r.NoError(err)
	defer fakeSrv.Close()

	// create a fake queue pinger. this is the simulated
	// scaler that pings the above fake interceptor
	ticker, pinger := newFakeQueuePinger(
		ctx,
		lggr,
		func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
		func(opts *fakeQueuePingerOpts) { opts.tickDur = 1 * time.Millisecond },
		func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
	)
	defer ticker.Stop()

	// sleep for more than enough time for the pinger to do its
	// first tick
	time.Sleep(5 * time.Millisecond)

	hdl := newImpl(lggr, pinger)
	res, err := hdl.GetMetrics(ctx, req)
	r.NoError(err)
	r.NotNil(res)
	r.Equal(1, len(res.MetricValues))
	metricVal := res.MetricValues[0]
	r.Equal(host, metricVal.MetricName)
	r.Equal(int64(pendingQLen), metricVal.MetricValue)
}

// Ensure that the queue pinger returns the aggregate request
// count when the host is set to "interceptor"
func TestGetMetricsInterceptorReturnsAggregate(t *testing.T) {
	const (
		ns          = "testns"
		svcName     = "testsrv"
		pendingQLen = 203
	)

	// create a request for the GetMetrics RPC call. it instructs
	// GetMetrics to return the counts for one specific host.
	// below, we do setup to ensure that we have a fake
	// interceptor, and that interceptor knows about the given host
	req := &externalscaler.GetMetricsRequest{
		ScaledObjectRef: &externalscaler.ScaledObjectRef{
			ScalerMetadata: map[string]string{
				"host": "interceptor",
			},
		},
	}

	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()

	// we need to create a new queuePinger with valid endpoints
	// to query this time, so that when counts are requested by
	// the internal queuePinger logic, there is a valid host from
	// which to request those counts
	q := queue.NewFakeCounter()
	// NOTE: don't call .Resize here or you'll have to make sure
	// to receive on q.ResizedCh
	q.RetMap["host1"] = pendingQLen
	q.RetMap["host2"] = pendingQLen

	// create a fake interceptor
	fakeSrv, fakeSrvURL, endpoints, err := startFakeQueueEndpointServer(
		ns,
		svcName,
		q,
		1,
	)
	r.NoError(err)
	defer fakeSrv.Close()

	// create a fake queue pinger. this is the simulated
	// scaler that pings the above fake interceptor
	ticker, pinger := newFakeQueuePinger(
		ctx,
		lggr,
		func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
		func(opts *fakeQueuePingerOpts) { opts.tickDur = 1 * time.Millisecond },
		func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
	)
	defer ticker.Stop()

	// sleep for more than enough time for the pinger to do its
	// first tick
	time.Sleep(5 * time.Millisecond)

	hdl := newImpl(lggr, pinger)
	res, err := hdl.GetMetrics(ctx, req)
	r.NoError(err)
	r.NotNil(res)
	r.Equal(1, len(res.MetricValues))
	metricVal := res.MetricValues[0]
	r.Equal("interceptor", metricVal.MetricName)
	aggregate := pinger.aggregate()
	r.Equal(int64(aggregate), metricVal.MetricValue)

}