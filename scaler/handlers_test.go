package main

import (
	context "context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	externalscaler "github.com/kedacore/http-add-on/proto"
)

func standardTarget() routing.Target {
	return routing.NewTarget(
		"testns",
		"testsrv",
		8080,
		"testdepl",
		123,
	)
}

func TestStreamIsActive(t *testing.T) {
	type testCase struct {
		name        string
		host        string
		expected    bool
		expectedErr bool
		setup       func(*routing.Table, *queuePinger)
	}
	r := require.New(t)
	testCases := []testCase{
		{
			name:        "Simple host inactive",
			host:        t.Name(),
			expected:    false,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
				q.pingMut.Lock()
				defer q.pingMut.Unlock()
				q.allCounts[t.Name()] = 0
			},
		},
		{
			name:        "Host is 'interceptor'",
			host:        "interceptor",
			expected:    true,
			expectedErr: false,
			setup:       func(*routing.Table, *queuePinger) {},
		},
		{
			name:        "Simple host active",
			host:        t.Name(),
			expected:    true,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
				q.pingMut.Lock()
				defer q.pingMut.Unlock()
				q.allCounts[t.Name()] = 1
			},
		},
		{
			name:        "No host present, but host in routing table",
			host:        t.Name(),
			expected:    false,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
			},
		},
		{
			name:        "Host doesn't exist",
			host:        t.Name(),
			expected:    false,
			expectedErr: true,
			setup:       func(*routing.Table, *queuePinger) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			table := routing.NewTable()
			ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(table, pinger)

			hdl := newImpl(
				lggr,
				pinger,
				table,
				123,
				200,
			)

			bufSize := 1024 * 1024
			lis := bufconn.Listen(bufSize)
			grpcServer := grpc.NewServer()
			defer grpcServer.Stop()
			externalscaler.RegisterExternalScalerServer(
				grpcServer,
				hdl,
			)
			go func() {
				r.NoError(grpcServer.Serve(lis))
			}()

			bufDialFunc := func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}

			conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialFunc), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := externalscaler.NewExternalScalerClient(conn)

			testRef := &externalscaler.ScaledObjectRef{
				ScalerMetadata: map[string]string{
					"host": tc.host,
				},
			}

			// First will see if we can establish the stream and handle this
			// error.
			streamClient, err := client.StreamIsActive(ctx, testRef)
			if err != nil {
				t.Fatalf("StreamIsActive failed: %v", err)
			}

			// Next, as in TestIsActive, we check for any error, expected
			// or unexpected, for each table test.
			res, err := streamClient.Recv()

			if tc.expectedErr && err != nil {
				return
			} else if err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}

			if tc.expected != res.Result {
				t.Fatalf("Expected IsActive result %v, got: %v", tc.expected, res.Result)
			}
		})
	}
}

func TestIsActive(t *testing.T) {
	type testCase struct {
		name        string
		host        string
		expected    bool
		expectedErr bool
		setup       func(*routing.Table, *queuePinger)
	}
	r := require.New(t)
	testCases := []testCase{
		{
			name:        "Simple host inactive",
			host:        t.Name(),
			expected:    false,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
				q.pingMut.Lock()
				defer q.pingMut.Unlock()
				q.allCounts[t.Name()] = 0
			},
		},
		{
			name:        "Host is 'interceptor'",
			host:        "interceptor",
			expected:    true,
			expectedErr: false,
			setup:       func(*routing.Table, *queuePinger) {},
		},
		{
			name:        "Simple host active",
			host:        t.Name(),
			expected:    true,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
				q.pingMut.Lock()
				defer q.pingMut.Unlock()
				q.allCounts[t.Name()] = 1
			},
		},
		{
			name:        "No host present, but host in routing table",
			host:        t.Name(),
			expected:    false,
			expectedErr: false,
			setup: func(table *routing.Table, q *queuePinger) {
				r.NoError(table.AddTarget(t.Name(), standardTarget()))
			},
		},
		{
			name:        "Host doesn't exist",
			host:        t.Name(),
			expected:    false,
			expectedErr: true,
			setup:       func(*routing.Table, *queuePinger) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			table := routing.NewTable()
			ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(table, pinger)
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
						"host": tc.host,
					},
				},
			)

			if tc.expectedErr && err != nil {
				return
			} else if err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
			if tc.expected != res.Result {
				t.Fatalf("Expected IsActive result %v, got: %v", tc.expected, res.Result)
			}
		})
	}
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
	r := require.New(t)
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
				r.NoError(ret.AddTarget("validHost", routing.NewTarget(
					ns,
					"testsrv",
					8080,
					"testdepl",
					123,
				)))
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
				r.NoError(ret.AddTarget("validHost", routing.NewTarget(
					ns,
					"testsrv",
					8080,
					"testdepl",
					123,
				)))
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
		{
			name: "host in routing table, missing in queue pinger",
			scalerMetadata: map[string]string{
				"host": "myhost.com",
			},
			setupFn: func(
				ctx context.Context,
				lggr logr.Logger,
			) (*routing.Table, *queuePinger, func(), error) {
				table := routing.NewTable()
				r := require.New(t)
				r.NoError(table.AddTarget(
					"myhost.com",
					standardTarget(),
				))
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
				r.Equal("myhost.com", metricVal.MetricName)
				// the value here needs to be the same thing as
				// the sum of the values in the fake queue created
				// in the setup function
				r.Equal(int64(0), metricVal.MetricValue)
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
