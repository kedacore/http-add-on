package main

import (
	context "context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"github.com/stretchr/testify/require"
)

func TestIsActive(t *testing.T) {
	const host = "TestIsActive.testing.com"
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	table := routing.NewTable()
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	pinger.pingMut.Lock()
	pinger.allCounts[host] = 0
	pinger.pingMut.Unlock()

	hdl := newImpl(
		lggr,
		pinger,
		table,
		123,
		200,
	)
	res, err := hdl.IsActive(
		ctx,
		&externalscaler.ScaledObjectRef{
			ScalerMetadata: map[string]string{
				"host": host,
			},
		},
	)
	r.NoError(err)
	r.NotNil(res)
	// initially, IsActive should return false since the
	// count for the host is 0
	r.False(res.Result)

	// incrment the count for the host and then expect
	// active to be true
	pinger.pingMut.Lock()
	pinger.allCounts[host]++
	pinger.pingMut.Unlock()
	res, err = hdl.IsActive(
		ctx,
		&externalscaler.ScaledObjectRef{
			ScalerMetadata: map[string]string{
				"host": host,
			},
		},
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
	table := routing.NewTable()
	table.AddTarget(host, routing.NewTarget(
		"testsrv",
		8080,
		"testdepl",
		int32(target),
	))
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	hdl := newImpl(lggr, pinger, table, 123, 200)
	meta := map[string]string{
		"host":                  host,
		"targetPendingRequests": strconv.Itoa(int(target)),
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

func TestGetMetrics(t *testing.T) {
	type testCase struct {
		name           string
		scalerMetadata map[string]string
		setupFn        func(
			context.Context,
			logr.Logger,
		) (*routing.Table, *queuePinger, func(), error)
		checkFn                        func(*testing.T, *externalscaler.GetMetricsResponse, error)
		defaultTargetMetric            int64
		defaultTargetMetricInterceptor int64
	}

	testCases := []testCase{
		{
			name:           "no 'host' field in the scaler metadata field",
			scalerMetadata: map[string]string{},
			setupFn: func(
				ctx context.Context,
				lggr logr.Logger,
			) (*routing.Table, *queuePinger, func(), error) {
				table := routing.NewTable()
				ticker, pinger := newFakeQueuePinger(ctx, lggr)
				return table, pinger, func() { ticker.Stop() }, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.Error(err)
				r.Nil(res)
				r.Contains(
					err.Error(),
					"no 'host' field found in ScaledObject metadata",
				)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
		{
			name: "missing host value in the queue pinger",
			scalerMetadata: map[string]string{
				"host": "missingHostInQueue",
			},
			setupFn: func(
				ctx context.Context,
				lggr logr.Logger,
			) (*routing.Table, *queuePinger, func(), error) {
				table := routing.NewTable()
				// create queue and ticker without the host in it
				ticker, pinger := newFakeQueuePinger(ctx, lggr)
				return table, pinger, func() { ticker.Stop() }, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.Error(err)
				r.Contains(err.Error(), "host 'missingHostInQueue' not found in counts")
				r.Nil(res)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
		{
			name: "valid host",
			scalerMetadata: map[string]string{
				"host": "validHost",
			},
			setupFn: func(
				ctx context.Context,
				lggr logr.Logger,
			) (*routing.Table, *queuePinger, func(), error) {
				table := routing.NewTable()

				// we need to create a new queuePinger with valid endpoints
				// to query this time, so that when counts are requested by
				// the internal queuePinger logic, there is a valid host from
				// which to request those counts
				q := queue.NewFakeCounter()
				// NOTE: don't call .Resize here or you'll have to make sure
				// to receive on q.ResizedCh
				q.RetMap["validHost"] = 201

				// create a fake interceptor
				fakeSrv, fakeSrvURL, endpoints, err := startFakeQueueEndpointServer(
					"testns",
					"testSvc",
					q,
					1,
				)
				if err != nil {
					return nil, nil, nil, err
				}

				// create a fake queue pinger. this is the simulated
				// scaler that pings the above fake interceptor
				const tickDur = 1 * time.Millisecond
				ticker, pinger := newFakeQueuePinger(
					ctx,
					lggr,
					func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
					func(opts *fakeQueuePingerOpts) { opts.tickDur = tickDur },
					func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
				)

				// sleep for a bit to ensure the pinger has time to do its first tick
				time.Sleep(10 * tickDur)

				return table, pinger, func() {
					ticker.Stop()
					fakeSrv.Close()
				}, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricValues))
				metricVal := res.MetricValues[0]
				r.Equal("validHost", metricVal.MetricName)
				r.Equal(int64(201), metricVal.MetricValue)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
		{
			name: "'interceptor' as host",
			scalerMetadata: map[string]string{
				"host": "interceptor",
			},
			setupFn: func(
				ctx context.Context,
				lggr logr.Logger,
			) (*routing.Table, *queuePinger, func(), error) {
				table := routing.NewTable()

				// we need to create a new queuePinger with valid endpoints
				// to query this time, so that when counts are requested by
				// the internal queuePinger logic, there is a valid host from
				// which to request those counts
				q := queue.NewFakeCounter()
				// NOTE: don't call .Resize here or you'll have to make sure
				// to receive on q.ResizedCh
				q.RetMap["host1"] = 201
				q.RetMap["host2"] = 202

				// create a fake interceptor
				fakeSrv, fakeSrvURL, endpoints, err := startFakeQueueEndpointServer(
					"testns",
					"testSvc",
					q,
					1,
				)
				if err != nil {
					return nil, nil, nil, err
				}

				// create a fake queue pinger. this is the simulated
				// scaler that pings the above fake interceptor
				const tickDur = 1 * time.Millisecond
				ticker, pinger := newFakeQueuePinger(
					ctx,
					lggr,
					func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
					func(opts *fakeQueuePingerOpts) { opts.tickDur = tickDur },
					func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
				)

				// sleep for a bit to ensure the pinger has time to do its first tick
				time.Sleep(4 * tickDur)

				return table, pinger, func() {
					ticker.Stop()
					fakeSrv.Close()
				}, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricValues))
				metricVal := res.MetricValues[0]
				r.Equal("interceptor", metricVal.MetricName)
				// the value here needs to be the same thing as
				// the sum of the values in the fake queue created
				// in the setup function
				r.Equal(int64(403), metricVal.MetricValue)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
	}

	for i, c := range testCases {
		tc := c
		name := fmt.Sprintf("test case %d: %s", i, tc.name)
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			ctx, done := context.WithCancel(
				context.Background(),
			)
			defer done()
			lggr := logr.Discard()
			table, pinger, cleanup, err := tc.setupFn(ctx, lggr)
			r.NoError(err)
			defer cleanup()
			hdl := newImpl(
				lggr,
				pinger,
				table,
				tc.defaultTargetMetric,
				tc.defaultTargetMetricInterceptor,
			)
			res, err := hdl.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
				ScaledObjectRef: &externalscaler.ScaledObjectRef{
					ScalerMetadata: tc.scalerMetadata,
				},
			})
			tc.checkFn(t, res, err)
		})
	}
}
