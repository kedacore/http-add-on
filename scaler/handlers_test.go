package main

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
)

const validHTTPScaledObjectName = "valid-httpscaledobject"

// newFakeClient creates a fake client populated with the given objects.
func newFakeClient(objs ...runtime.Object) client.Reader {
	scheme := runtime.NewScheme()
	utilruntime.Must(httpv1alpha1.AddToScheme(scheme))
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
}

// newDefaultHTTPScaledObject creates a default HTTPScaledObject for testing.
func newDefaultHTTPScaledObject(namespace string, scalingMetric *httpv1alpha1.ScalingMetricSpec) *httpv1alpha1.HTTPScaledObject {
	return &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      validHTTPScaledObjectName,
			Namespace: namespace,
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScalingMetric: scalingMetric,
		},
	}
}

func TestStreamIsActive(t *testing.T) {
	type testCase struct {
		name           string
		expected       bool
		expectedErr    bool
		setup          func(t *testing.T, qp *queuePinger)
		scalerMetadata map[string]string
		scalingMetric  *httpv1alpha1.ScalingMetricSpec
	}
	testCases := []testCase{
		{
			name:        "Simple host inactive",
			expected:    false,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:        "Simple host active by concurrency and scaling metric nil",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
			scalingMetric: nil,
		},
		{
			name:        "Simple host active by concurrency and concurrency scaling metric",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
			scalingMetric: &httpv1alpha1.ScalingMetricSpec{
				Concurrency: &httpv1alpha1.ConcurrencyMetricSpec{},
			},
		},
		{
			name:        "Simple host active by concurrency and concurrency scaling rate",
			expected:    false,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
			scalingMetric: &httpv1alpha1.ScalingMetricSpec{
				Rate: &httpv1alpha1.RateMetricSpec{},
			},
		},
		{
			name:        "Simple host active by rate and concurrency scaling metric",
			expected:    false,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					RPS: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
			scalingMetric: &httpv1alpha1.ScalingMetricSpec{
				Concurrency: &httpv1alpha1.ConcurrencyMetricSpec{},
			},
		},
		{
			name:        "Simple host active by concurrency and concurrency scaling rate",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					RPS: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
			scalingMetric: &httpv1alpha1.ScalingMetricSpec{
				Rate: &httpv1alpha1.RateMetricSpec{},
			},
		},
		{
			name:        "Simple multi host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 2,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:        "Host doesn't exist",
			expected:    false,
			expectedErr: true,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
		{
			name:        "Interceptor",
			expected:    true,
			expectedErr: false,
			setup: func(_ *testing.T, qp *queuePinger) {
				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts["a"] = queue.Count{
					Concurrency: 1,
				}
				qp.allCounts["b"] = queue.Count{
					Concurrency: 2,
				}
				qp.allCounts["c"] = queue.Count{
					Concurrency: 3,
				}
			},
			scalerMetadata: map[string]string{
				keyInterceptorTargetPendingRequests: "1000",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			fakeClient := newFakeClient(newDefaultHTTPScaledObject("default", tc.scalingMetric))
			ticker, pinger, err := newFakeQueuePinger(lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(t, pinger)

			hdl := newScalerHandler(
				lggr,
				pinger,
				fakeClient,
				200*time.Millisecond,
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
				assert.NoError(t, grpcServer.Serve(lis))
			}()

			bufDialFunc := func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}

			conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(bufDialFunc), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := externalscaler.NewExternalScalerClient(conn)

			testRef := &externalscaler.ScaledObjectRef{
				Namespace:      "default",
				Name:           t.Name(),
				ScalerMetadata: tc.scalerMetadata,
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
		name           string
		expected       bool
		expectedErr    bool
		setup          func(t *testing.T, qp *queuePinger)
		scalerMetadata map[string]string
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

				qp.allCounts[key] = queue.Count{
					Concurrency: 0,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:        "Simple host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 1,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:        "Simple multi host active",
			expected:    true,
			expectedErr: false,
			setup: func(t *testing.T, qp *queuePinger) {
				namespacedName := &types.NamespacedName{
					Namespace: "default",
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts[key] = queue.Count{
					Concurrency: 2,
				}
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:        "Host doesn't exist",
			expected:    false,
			expectedErr: true,
			setup:       func(_ *testing.T, _ *queuePinger) {},
		},
		{
			name:        "Interceptor",
			expected:    true,
			expectedErr: false,
			setup: func(_ *testing.T, qp *queuePinger) {
				qp.pingMut.Lock()
				defer qp.pingMut.Unlock()

				qp.allCounts["a"] = queue.Count{
					Concurrency: 1,
				}
				qp.allCounts["b"] = queue.Count{
					Concurrency: 2,
				}
				qp.allCounts["c"] = queue.Count{
					Concurrency: 3,
				}
			},
			scalerMetadata: map[string]string{
				keyInterceptorTargetPendingRequests: "1000",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			lggr := logr.Discard()
			fakeClient := newFakeClient(newDefaultHTTPScaledObject("default", nil))
			ticker, pinger, err := newFakeQueuePinger(lggr)
			r.NoError(err)
			defer ticker.Stop()
			tc.setup(t, pinger)
			hdl := newScalerHandler(
				lggr,
				pinger,
				fakeClient,
				200*time.Millisecond,
			)

			res, err := hdl.IsActive(
				ctx,
				&externalscaler.ScaledObjectRef{
					Namespace:      "default",
					Name:           t.Name(),
					ScalerMetadata: tc.scalerMetadata,
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
		name           string
		httpso         *httpv1alpha1.HTTPScaledObject
		checker        func(*testing.T, *externalscaler.GetMetricSpecResponse, error)
		scalerMetadata map[string]string
	}
	cases := []testCase{
		{
			name: "valid host as single host value in scaler metadata",
			httpso: &httpv1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: validHTTPScaledObjectName},
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					ScaleTargetRef:        httpv1alpha1.ScaleTargetRef{Name: "testdepl", Service: "testsrv", Port: 8080},
					TargetPendingRequests: ptr.To[int32](123),
				},
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricSpecs, 1)
				spec := res.MetricSpecs[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: validHTTPScaledObjectName}), spec.MetricName)
				r.Equal(int64(123), spec.TargetSize)
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name: "valid hosts as multiple hosts value in scaler metadata",
			httpso: &httpv1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: validHTTPScaledObjectName},
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{"validHost1", "validHost2"},
					ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
						Name:    "testdepl",
						Service: "testsrv",
						Port:    8080,
					},
					TargetPendingRequests: ptr.To[int32](123),
				},
			},
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricSpecs, 1)
				spec := res.MetricSpecs[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: validHTTPScaledObjectName}), spec.MetricName)
				r.Equal(int64(123), spec.TargetSize)
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name:   "interceptor",
			httpso: nil, // No httpso needed for interceptor case
			checker: func(t *testing.T, res *externalscaler.GetMetricSpecResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricSpecs, 1)
				spec := res.MetricSpecs[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), spec.MetricName)
				r.Equal(int64(1000), spec.TargetSize)
			},
			scalerMetadata: map[string]string{
				keyInterceptorTargetPendingRequests: "1000",
			},
		},
	}

	for i, tc := range cases {
		testName := fmt.Sprintf("test case #%d: %s", i, tc.name)
		t.Run(testName, func(t *testing.T) {
			ctx := context.Background()
			t.Parallel()
			lggr := logr.Discard()

			var fakeClient client.Reader
			if tc.httpso != nil {
				fakeClient = newFakeClient(tc.httpso)
			} else {
				fakeClient = newFakeClient()
			}

			ticker, pinger, err := newFakeQueuePinger(lggr)
			if err != nil {
				t.Fatalf("error creating new fake queue pinger: %s", err)
			}
			defer ticker.Stop()

			hdl := newScalerHandler(lggr, pinger, fakeClient, 200*time.Millisecond)
			scaledObjectRef := externalscaler.ScaledObjectRef{
				Namespace:      ns,
				Name:           t.Name(),
				ScalerMetadata: tc.scalerMetadata,
			}
			res, err := hdl.GetMetricSpec(ctx, &scaledObjectRef)
			tc.checker(t, res, err)
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
			context.Context,
			logr.Logger,
		) (client.Reader, *queuePinger, func(), error)
		checkFn        func(*testing.T, *externalscaler.GetMetricsResponse, error)
		scalerMetadata map[string]string
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
			q.RetMap[host] = queue.Count{
				Concurrency: val,
			}
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
			lggr,
			func(opts *fakeQueuePingerOpts) { opts.endpoints = endpoints },
			func(opts *fakeQueuePingerOpts) { opts.tickDur = queuePingerTickDur },
			func(opts *fakeQueuePingerOpts) { opts.port = fakeSrvURL.Port() },
		)
		if err != nil {
			return nil, nil, err
		}

		go func() {
			_ = pinger.start(ctx, ticker, k8s.NewFakeEndpointsCache())
		}()
		// sleep for a bit to ensure the pinger has time to do its first tick
		time.Sleep(10 * queuePingerTickDur)
		return pinger, func() {
			ticker.Stop()
			fakeSrv.Close()
			ctx.Done()
		}, nil
	}

	testCases := []testCase{
		{
			name: "HTTPSO missing in the queue pinger",
			setupFn: func(
				t *testing.T,
				ctx context.Context,
				lggr logr.Logger,
			) (client.Reader, *queuePinger, func(), error) {
				fakeClient := newFakeClient(newDefaultHTTPScaledObject(ns, nil))

				// create queue and ticker without the host in it
				ticker, pinger, err := newFakeQueuePinger(lggr)
				if err != nil {
					return nil, nil, nil, err
				}

				return fakeClient, pinger, func() { ticker.Stop() }, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricValues, 1)
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: validHTTPScaledObjectName}), metricVal.MetricName)
				r.Equal(int64(0), metricVal.MetricValue)
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name: "HTTPSO present in the queue pinger",
			setupFn: func(
				t *testing.T,
				ctx context.Context,
				lggr logr.Logger,
			) (client.Reader, *queuePinger, func(), error) {
				fakeClient := newFakeClient(newDefaultHTTPScaledObject(ns, nil))

				namespacedName := &types.NamespacedName{
					Namespace: ns,
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					key: 201,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return fakeClient, pinger, done, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricValues, 1)
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: validHTTPScaledObjectName}), metricVal.MetricName)
				r.Equal(int64(201), metricVal.MetricValue)
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name: "multiple validHosts add MetricValues",
			setupFn: func(
				t *testing.T,
				ctx context.Context,
				lggr logr.Logger,
			) (client.Reader, *queuePinger, func(), error) {
				fakeClient := newFakeClient(newDefaultHTTPScaledObject(ns, nil))

				namespacedName := &types.NamespacedName{
					Namespace: ns,
					Name:      validHTTPScaledObjectName,
				}
				key := namespacedName.String()

				pinger, done, err := startFakeInterceptorServer(ctx, lggr, map[string]int{
					key: 579,
				}, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return fakeClient, pinger, done, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricValues, 1)
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: validHTTPScaledObjectName}), metricVal.MetricName)
				// the value here needs to be the same thing as
				// the sum of the values in the fake queue created
				// in the setup function
				r.Equal(int64(579), metricVal.MetricValue)
			},
			scalerMetadata: map[string]string{
				k8s.HTTPScaledObjectKey: validHTTPScaledObjectName,
			},
		},
		{
			name: "interceptor",
			setupFn: func(
				t *testing.T,
				ctx context.Context,
				lggr logr.Logger,
			) (client.Reader, *queuePinger, func(), error) {
				fakeClient := newFakeClient(newDefaultHTTPScaledObject(ns, nil))

				memory := map[string]int{
					"a": 1,
					"b": 2,
					"c": 3,
				}
				pinger, done, err := startFakeInterceptorServer(ctx, lggr, memory, 2*time.Millisecond)
				if err != nil {
					return nil, nil, nil, err
				}

				return fakeClient, pinger, done, nil
			},
			checkFn: func(t *testing.T, res *externalscaler.GetMetricsResponse, err error) {
				t.Helper()
				r := require.New(t)
				r.NoError(err)
				r.NotNil(res)
				r.Len(res.MetricValues, 1)
				metricVal := res.MetricValues[0]
				r.Equal(MetricName(&types.NamespacedName{Namespace: ns, Name: t.Name()}), metricVal.MetricName)
				// the value here needs to be the same thing as
				// the sum of the values in the fake queue created
				// in the setup function
				r.Equal(int64(6), metricVal.MetricValue)
			},
			scalerMetadata: map[string]string{
				keyInterceptorTargetPendingRequests: "1000",
			},
		},
	}

	for i, c := range testCases {
		tc := c
		name := fmt.Sprintf("test case %d: %s", i, tc.name)
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			ctx := t.Context()
			lggr := logr.Discard()
			fakeClient, pinger, cleanup, err := tc.setupFn(t, ctx, lggr)
			r.NoError(err)
			defer cleanup()
			hdl := newScalerHandler(
				lggr,
				pinger,
				fakeClient,
				200*time.Millisecond,
			)
			res, err := hdl.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
				ScaledObjectRef: &externalscaler.ScaledObjectRef{
					Namespace:      ns,
					Name:           t.Name(),
					ScalerMetadata: tc.scalerMetadata,
				},
			})
			tc.checkFn(t, res, err)
		})
	}
}
