package main

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	informersexternalversionshttpv1alpha1mock "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1/mock"
	listershttpv1alpha1mock "github.com/kedacore/http-add-on/operator/generated/listers/http/v1alpha1/mock"
	"github.com/kedacore/http-add-on/pkg/queue"
)

func TestStreamIsActive(t *testing.T) {
	type testCase struct {
		name        string
		hosts       string
		expected    bool
		expectedErr bool
		setup       func(t *testing.T, qp *queuePinger)
	}
	testCases := []testCase{
		{
			name:        "Simple host inactive",
			expected:    false,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 0
			},
		},
		{
			name:        "Simple host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 1
			},
		},
		{
			name:        "Simple multi host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 2
			},
		},
		{
			name:        "No host present, but host in routing table",
			expected:    false,
			expectedErr: false,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
		{
			name:        "Host doesn't exist",
			expected:    false,
			expectedErr: true,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			informer, _, _ := newMocks(ctrl)
			ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(t, pinger)

			hdl := newImpl(
				lggr,
				pinger,
				informer,
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
				Namespace: "default",
				Name:      t.Name(),
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
			}
			if err != nil {
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
		expected    bool
		expectedErr bool
		setup       func(t *testing.T, qp *queuePinger)
	}
	testCases := []testCase{
		{
			name:        "Simple host inactive",
			expected:    false,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 0
			},
		},
		{
			name:        "Simple host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 1
			},
		},
		{
			name:        "Simple multi host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      t.Name(),
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = 2
			},
		},
		{
			name:        "No host present, but host in routing table",
			expected:    false,
			expectedErr: false,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
		{
			name:        "Host doesn't exist",
			expected:    false,
			expectedErr: true,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			informer, _, _ := newMocks(ctrl)
			ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(t, pinger)
			hdl := newImpl(
				lggr,
				pinger,
				informer,
				123,
				200,
			)

			res, err := hdl.IsActive(
				ctx,
				&externalscaler.ScaledObjectRef{
					Namespace: "default",
					Name:      t.Name(),
				},
			)

			if tc.expectedErr && err != nil {
				return
			}
			if err != nil {
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
		newInformer                    func(*testing.T, *gomock.Controller) *informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer
		checker                        func(*testing.T, *externalscaler.GetMetricSpecResponse, error)
	}
	cases := []testCase{
		{
			name:                           "valid host as single host value in scaler metadata",
			defaultTargetMetric:            0,
			defaultTargetMetricInterceptor: 123,
			newInformer: func(t *testing.T, ctrl *gomock.Controller) *informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer {
				informer, _, namespaceLister := newMocks(ctrl)

				httpso := &httpv1alpha1.HTTPScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      t.Name(),
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						ScaleTargetRef: &httpv1alpha1.ScaleTargetRef{
							Deployment: "testdepl",
							Service:    "testsrv",
							Port:       8080,
						},
						TargetPendingRequests: pointer.Int32(123),
					},
				}
				namespaceLister.EXPECT().
					Get(httpso.GetName()).
					Return(httpso, nil)

				return informer
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricSpecs))
				spec := res.MetricSpecs[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), spec.MetricName)
				r.Equal(int64(123), spec.TargetSize)
			},
		},
		{
			name:                           "valid hosts as multiple hosts value in scaler metadata",
			defaultTargetMetric:            0,
			defaultTargetMetricInterceptor: 123,
			newInformer: func(t *testing.T, ctrl *gomock.Controller) *informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer {
				informer, _, namespaceLister := newMocks(ctrl)

				httpso := &httpv1alpha1.HTTPScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
						Name:      t.Name(),
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"validHost1",
							"validHost2",
						},
						ScaleTargetRef: &httpv1alpha1.ScaleTargetRef{
							Deployment: "testdepl",
							Service:    "testsrv",
							Port:       8080,
						},
						TargetPendingRequests: pointer.Int32(123),
					},
				}
				namespaceLister.EXPECT().
					Get(httpso.GetName()).
					Return(httpso, nil)

				return informer
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricSpecs))
				spec := res.MetricSpecs[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), spec.MetricName)
				r.Equal(int64(123), spec.TargetSize)
			},
		},
	}

	for i, c := range cases {
		testName := fmt.Sprintf("test case #%d: %s", i, c.name)
		// capture tc in scope so that we can run the below test
		// in parallel
		testCase := c
		t.Run(testName, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			t.Parallel()
			lggr := logr.Discard()
			informer := testCase.newInformer(t, ctrl)
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
				informer,
				testCase.defaultTargetMetric,
				testCase.defaultTargetMetricInterceptor,
			)
			scaledObjectRef := externalscaler.ScaledObjectRef{
				Namespace: ns,
				Name:      t.Name(),
			}
			ret, err := hdl.GetMetricSpec(ctx, &scaledObjectRef)
			testCase.checker(t, ret, err)
		})
	}
}

func TestGetMetrics(t *testing.T) {
	const (
		ns = "default"
	)

	type testCase struct {
		name    string
		setupFn func(
			*testing.T,
			*gomock.Controller,
			context.Context,
			logr.Logger,
		) (*informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer, *queuePinger, func(), error)
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
			name: "HTTPSO missing in the queue pinger",
			setupFn: func(
				t *testing.T,
				ctrl *gomock.Controller,
				ctx context.Context,
				lggr logr.Logger,
			) (*informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer, *queuePinger, func(), error) {
				informer, _, _ := newMocks(ctrl)

				// create queue and ticker without the host in it
				ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
				if err != nil {
					return nil, nil, nil, err
				}

				return informer, pinger, func() { ticker.Stop() }, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricValues))
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), metricVal.MetricName)
				r.Equal(int64(0), metricVal.MetricValue)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
		{
			name: "HTTPSO present in the queue pinger",
			setupFn: func(
				t *testing.T,
				ctrl *gomock.Controller,
				ctx context.Context,
				lggr logr.Logger,
			) (*informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer, *queuePinger, func(), error) {
				informer, _, _ := newMocks(ctrl)

				namespacedName := &types.NamespacedName{
					Namespace: ns,
					Name:      t.Name(),
				}
				key := namespacedName.String()

				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					key: 201,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return informer, pinger, done, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricValues))
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), metricVal.MetricName)
				r.Equal(int64(201), metricVal.MetricValue)
			},
			defaultTargetMetric:            int64(200),
			defaultTargetMetricInterceptor: int64(300),
		},
		{
			name: "multiple validHosts add MetricValues",
			setupFn: func(
				t *testing.T,
				ctrl *gomock.Controller,
				ctx context.Context,
				lggr logr.Logger,
			) (*informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer, *queuePinger, func(), error) {
				informer, _, _ := newMocks(ctrl)

				namespacedName := &types.NamespacedName{
					Namespace: ns,
					Name:      t.Name(),
				}
				key := namespacedName.String()

				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					key: 579,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return informer, pinger, done, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Equal(1, len(res.MetricValues))
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), metricVal.MetricName)
				// the value here needs to be the same thing as
				// the sum of the values in the fake queue created
				// in the setup function
				r.Equal(int64(579), metricVal.MetricValue)
			},
			defaultTargetMetric:            int64(500),
			defaultTargetMetricInterceptor: int64(600),
		},
	}

	for i, c := range testCases {
		tc := c
		name := fmt.Sprintf("test case %d: %s", i, tc.name)
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := require.New(t)
			ctx, done := context.WithCancel(
				context.Background(),
			)
			defer done()
			lggr := logr.Discard()
			informer, pinger, cleanup, err := tc.setupFn(t, ctrl, ctx, lggr)
			r.NoError(err)
			defer cleanup()
			hdl := newImpl(
				lggr,
				pinger,
				informer,
				tc.defaultTargetMetric,
				tc.defaultTargetMetricInterceptor,
			)
			res, err := hdl.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
				ScaledObjectRef: &externalscaler.ScaledObjectRef{
					Namespace: ns,
					Name:      t.Name(),
				},
			})
			tc.checkFn(t, res, err)
		})
	}
}

func newMocks(ctrl *gomock.Controller) (*informersexternalversionshttpv1alpha1mock.MockHTTPScaledObjectInformer, *listershttpv1alpha1mock.MockHTTPScaledObjectLister, *listershttpv1alpha1mock.MockHTTPScaledObjectNamespaceLister) {
	namespaceLister := listershttpv1alpha1mock.NewMockHTTPScaledObjectNamespaceLister(ctrl)
	namespaceLister.EXPECT().
		Get("").
		DoAndReturn(func(name string) (*httpv1alpha1.HTTPScaledObject, error) {
			return nil, errors.NewNotFound(httpv1alpha1.Resource("httpscaledobject"), name)
		}).
		AnyTimes()

	lister := listershttpv1alpha1mock.NewMockHTTPScaledObjectLister(ctrl)
	lister.EXPECT().
		HTTPScaledObjects(gomock.Any()).
		Return(namespaceLister).
		AnyTimes()

	informer := informersexternalversionshttpv1alpha1mock.NewMockHTTPScaledObjectInformer(ctrl)
	informer.EXPECT().
		Lister().
		Return(lister).
		AnyTimes()

	return informer, lister, namespaceLister
}
