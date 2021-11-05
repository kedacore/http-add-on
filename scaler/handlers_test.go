package main

import (
	context "context"
	"fmt"
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
	ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
	r.NoError(err)
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

func TestGetMetricSpecTable(t *testing.T) {
	const ns = "testns"
	type testCase struct {
		name                           string
		defaultTargetMetric            int64
		defaultTargetMetricInterceptor int64
		scalerMetadata                 map[string]string
		newRoutingTableFn              func() *routing.Table
		checker                        func(*testing.T, *externalscaler.GetMetricSpecResponse, error)
	}

	cases := []testCase{
		{
			name:                           "valid host as host value in scaler metadata",
			defaultTargetMetric:            0,
			defaultTargetMetricInterceptor: 123,
			scalerMetadata: map[string]string{
				"host":                  "validHost",
				"targetPendingRequests": "123",
			},
			newRoutingTableFn: func() *routing.Table {
				ret := routing.NewTable()
				ret.AddTarget("validHost", routing.NewTarget(
					ns,
					"testsrv",
					8080,
					"testdepl",
					123,
				))
				return ret
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricSpecs))
				spec := res.MetricSpecs[0]
				r.Equal("validHost", spec.MetricName)
				r.Equal(int64(123), spec.TargetSize)
			},
		},
		{
			name:                           "interceptor as host in scaler metadata",
			defaultTargetMetric:            1000,
			defaultTargetMetricInterceptor: 2000,
			scalerMetadata: map[string]string{
				"host":                  "interceptor",
				"targetPendingRequests": "123",
			},
			newRoutingTableFn: func() *routing.Table {
				ret := routing.NewTable()
				ret.AddTarget("validHost", routing.NewTarget(
					ns,
					"testsrv",
					8080,
					"testdepl",
					123,
				))
				return ret
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricSpecs))
				spec := res.MetricSpecs[0]
				r.Equal("interceptor", spec.MetricName)
				r.Equal(int64(2000), spec.TargetSize)
			},
		},
	}

	for i, c := range cases {
		testName := fmt.Sprintf("test case #%d: %s", i, c.name)
		// capture tc in scope so that we can run the below test
		// in parallel
		testCase := c
		t.Run(testName, func(t *testing.T) {
			ctx := context.Background()
			t.Parallel()
			lggr := logr.Discard()
			table := testCase.newRoutingTableFn()
			ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
			if err != nil {
				t.Fatalf(
					"error creating new fake queue pinger and related components: %s",
					err,
				)
			}
			defer ticker.Stop()
			hdl := newImpl(
				lggr,
				pinger,
				table,
				testCase.defaultTargetMetric,
				testCase.defaultTargetMetricInterceptor,
			)
			scaledObjectRef := externalscaler.ScaledObjectRef{
				ScalerMetadata: testCase.scalerMetadata,
			}
			ret, err := hdl.GetMetricSpec(ctx, &scaledObjectRef)
			testCase.checker(t, ret, err)
		})
	}
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

	startFakeInterceptorServer := func(
		ctx context.Context,
		lggr logr.Logger,
		hostMap map[string]int,
		queuePingerTickDur time.Duration,
	) (*queuePinger, func(), error) {
		// create a new fake queue with the host map in it
		q := queue.NewFakeCounter()
		for host, val := range hostMap {
			// NOTE: don't call .Resize here or you'll have to make sure
			// to receive on q.ResizedCh
			q.RetMap[host] = val
		}
		// create the HTTP server to encode and serve
		// the host map
		fakeSrv, fakeSrvURL, endpoints, err := startFakeQueueEndpointServer(
			"testns",
			"testSvc",
			q,
			1,
		)
		if err != nil {
			return nil, nil, err
		}

		// create a fake queue pinger. this is the simulated
		// scaler that pings the above fake interceptor
		ticker, pinger, err := newFakeQueuePinger(
			ctx,
			lggr,
			func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
			func(opts *fakeQueuePingerOpts) { opts.tickDur = queuePingerTickDur },
			func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
		)
		if err != nil {
			return nil, nil, err
		}

		// sleep for a bit to ensure the pinger has time to do its first tick
		time.Sleep(10 * queuePingerTickDur)
		return pinger, func() {
			ticker.Stop()
			fakeSrv.Close()
		}, nil
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
				ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
				if err != nil {
					return nil, nil, nil, err
				}
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
				ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
				if err != nil {
					return nil, nil, nil, err
				}
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
				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					"validHost": 201,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return table, pinger, done, nil
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
				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					"host1": 201,
					"host2": 202,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}
				return table, pinger, done, nil
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
