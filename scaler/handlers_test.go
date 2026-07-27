package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

var (
	testIRName              = "test-ir"
	testIRNamespace         = "test-ns"
	testIRConcurrencyMetric = ConcurrencyMetricName(testIRName)
	testIRRateMetric        = RateMetricName(testIRName)
	testIRKey               = types.NamespacedName{Name: testIRName, Namespace: testIRNamespace}
	testScaledObjectRef     = &externalscaler.ScaledObjectRef{
		Name:      "test-so",
		Namespace: testIRNamespace,
		ScalerMetadata: map[string]string{
			k8s.InterceptorRouteKey: testIRName,
		},
	}
)

// newFakeClient creates a fake client populated with the given objects.
func newFakeClient(objs ...runtime.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(cache.NewScheme()).
		WithRuntimeObjects(objs...).
		Build()
}

func newTestInterceptorRoute(scalingMetric httpv1beta1.ScalingMetricSpec) *httpv1beta1.InterceptorRoute {
	return &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testIRName,
			Namespace: testIRNamespace,
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "test-svc",
				Port:    8080,
			},
			ScalingMetric: scalingMetric,
		},
	}
}

func newTestScalerHandler(t *testing.T, ir *httpv1beta1.InterceptorRoute, count aggregatedCount) *scalerHandler {
	t.Helper()

	hdl, _ := newTestScalerHandlerWithClient(t, ir, count)
	return hdl
}

// newTestScalerHandlerWithClient also returns the client backing the handler,
// so that tests can modify the InterceptorRoute after the handler was created.
func newTestScalerHandlerWithClient(t *testing.T, ir *httpv1beta1.InterceptorRoute, count aggregatedCount) (*scalerHandler, client.Client) {
	t.Helper()

	var cl client.Client
	if ir != nil {
		cl = newFakeClient(ir)
	} else {
		cl = newFakeClient()
	}

	ticker, pinger, err := newFakeQueuePinger(logr.Discard())
	if err != nil {
		t.Fatalf("creating fake queue pinger: %v", err)
	}
	t.Cleanup(ticker.Stop)

	if ir != nil {
		key := k8s.ResourceKey(ir.Namespace, ir.Name)
		pinger.allCounts[key] = count
	}

	return newScalerHandler(logr.Discard(), pinger, cl, time.Second), cl
}

func TestGetMetricSpec(t *testing.T) {
	tests := map[string]struct {
		scalingMetric httpv1beta1.ScalingMetricSpec
		want          []*externalscaler.MetricSpec
	}{
		"concurrency only": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 50},
			},
			want: []*externalscaler.MetricSpec{
				{MetricName: testIRConcurrencyMetric, TargetSizeFloat: 50},
			},
		},
		"rate only": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 200},
			},
			want: []*externalscaler.MetricSpec{
				{MetricName: testIRRateMetric, TargetSizeFloat: 200},
			},
		},
		"both concurrency and rate": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 50},
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 200},
			},
			want: []*externalscaler.MetricSpec{
				{MetricName: testIRConcurrencyMetric, TargetSizeFloat: 50},
				{MetricName: testIRRateMetric, TargetSizeFloat: 200},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ir := newTestInterceptorRoute(tc.scalingMetric)
			hdl := newTestScalerHandler(t, ir, aggregatedCount{})

			resp, err := hdl.GetMetricSpec(t.Context(), testScaledObjectRef)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			specs := resp.GetMetricSpecs()
			if got, want := len(specs), len(tc.want); got != want {
				t.Fatalf("got %d metric specs, want %d", got, want)
			}
			for i, spec := range specs {
				if got, want := spec.MetricName, tc.want[i].MetricName; got != want {
					t.Errorf("specs[%d].MetricName = %q, want %q", i, got, want)
				}
				if got, want := spec.TargetSizeFloat, tc.want[i].TargetSizeFloat; got != want {
					t.Errorf("specs[%d].TargetSizeFloat = %v, want %v", i, got, want)
				}
			}
		})
	}

	t.Run("IR not found", func(t *testing.T) {
		hdl := newTestScalerHandler(t, nil, aggregatedCount{})

		_, err := hdl.GetMetricSpec(t.Context(), testScaledObjectRef)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetMetrics(t *testing.T) {
	tests := map[string]struct {
		scalingMetric httpv1beta1.ScalingMetricSpec
		count         aggregatedCount
		want          []*externalscaler.MetricValue
	}{
		"concurrency value": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
			},
			count: aggregatedCount{Concurrency: 42},
			want: []*externalscaler.MetricValue{
				{MetricName: testIRConcurrencyMetric, MetricValueFloat: 42},
			},
		},
		"rate value": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 100},
			},
			count: aggregatedCount{RequestRate: 15.5},
			want: []*externalscaler.MetricValue{
				{MetricName: testIRRateMetric, MetricValueFloat: 15.5},
			},
		},
		"both metrics": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 200},
			},
			count: aggregatedCount{Concurrency: 10, RequestRate: 5.5},
			want: []*externalscaler.MetricValue{
				{MetricName: testIRConcurrencyMetric, MetricValueFloat: 10},
				{MetricName: testIRRateMetric, MetricValueFloat: 5.5},
			},
		},
		"no traffic": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
			},
			want: []*externalscaler.MetricValue{
				{MetricName: testIRConcurrencyMetric, MetricValueFloat: 0},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ir := newTestInterceptorRoute(tc.scalingMetric)
			hdl := newTestScalerHandler(t, ir, tc.count)

			req := &externalscaler.GetMetricsRequest{
				ScaledObjectRef: testScaledObjectRef,
			}
			resp, err := hdl.GetMetrics(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			values := resp.GetMetricValues()
			if got, want := len(values), len(tc.want); got != want {
				t.Fatalf("got %d metric values, want %d", got, want)
			}
			for i, v := range values {
				if got, want := v.MetricName, tc.want[i].MetricName; got != want {
					t.Errorf("values[%d].MetricName = %q, want %q", i, got, want)
				}
				if got, want := v.MetricValueFloat, tc.want[i].MetricValueFloat; got != want {
					t.Errorf("values[%d].MetricValueFloat = %v, want %v", i, got, want)
				}
			}
		})
	}

	t.Run("IR not found", func(t *testing.T) {
		hdl := newTestScalerHandler(t, nil, aggregatedCount{})

		req := &externalscaler.GetMetricsRequest{
			ScaledObjectRef: testScaledObjectRef,
		}
		_, err := hdl.GetMetrics(t.Context(), req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("filters by requested metric name", func(t *testing.T) {
		ir := newTestInterceptorRoute(httpv1beta1.ScalingMetricSpec{
			Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 3},
			RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 4},
		})
		hdl := newTestScalerHandler(t, ir, aggregatedCount{Concurrency: 12, RequestRate: 24})

		tests := map[string]struct {
			metricName string
			want       []*externalscaler.MetricValue
		}{
			"request concurrency only": {
				metricName: testIRConcurrencyMetric,
				want: []*externalscaler.MetricValue{
					{MetricName: testIRConcurrencyMetric, MetricValueFloat: 12},
				},
			},
			"request rate only": {
				metricName: testIRRateMetric,
				want: []*externalscaler.MetricValue{
					{MetricName: testIRRateMetric, MetricValueFloat: 24},
				},
			},
			"empty metric name returns all": {
				metricName: "",
				want: []*externalscaler.MetricValue{
					{MetricName: testIRConcurrencyMetric, MetricValueFloat: 12},
					{MetricName: testIRRateMetric, MetricValueFloat: 24},
				},
			},
			"unknown metric name returns empty": {
				metricName: "http_test-ir_unknown",
				want:       []*externalscaler.MetricValue{},
			},
		}
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				req := &externalscaler.GetMetricsRequest{
					ScaledObjectRef: testScaledObjectRef,
					MetricName:      tc.metricName,
				}
				resp, err := hdl.GetMetrics(t.Context(), req)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				values := resp.GetMetricValues()
				if got, want := len(values), len(tc.want); got != want {
					t.Fatalf("got %d metric values, want %d", got, want)
				}

				for i, v := range values {
					if got, want := v.MetricName, tc.want[i].MetricName; got != want {
						t.Errorf("values[%d].MetricName = %q, want %q", i, got, want)
					}
					if got, want := v.MetricValueFloat, tc.want[i].MetricValueFloat; got != want {
						t.Errorf("values[%d].MetricValueFloat = %v, want %v", i, got, want)
					}
				}
			})
		}
	})
}

func TestIsActive(t *testing.T) {
	tests := map[string]struct {
		scalingMetric httpv1beta1.ScalingMetricSpec
		count         aggregatedCount
		wantActive    bool
	}{
		"active by concurrency": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
			},
			count:      aggregatedCount{Concurrency: 5},
			wantActive: true,
		},
		"active by rate": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 100},
			},
			count:      aggregatedCount{RequestRate: 3.5},
			wantActive: true,
		},
		"inactive zero traffic": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
			},
			wantActive: false,
		},
		"active for one of two metrics": {
			scalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
				RequestRate: &httpv1beta1.RequestRateTargetSpec{TargetValue: 200},
			},
			count:      aggregatedCount{Concurrency: 1, RequestRate: 0},
			wantActive: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ir := newTestInterceptorRoute(tc.scalingMetric)
			hdl := newTestScalerHandler(t, ir, tc.count)

			resp, err := hdl.IsActive(t.Context(), testScaledObjectRef)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := resp.Result, tc.wantActive; got != want {
				t.Errorf("IsActive result = %v, want %v", got, want)
			}
		})
	}

	t.Run("IR not found", func(t *testing.T) {
		hdl := newTestScalerHandler(t, nil, aggregatedCount{})

		_, err := hdl.IsActive(t.Context(), testScaledObjectRef)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestStreamIsActive(t *testing.T) {
	scalingMetric := httpv1beta1.ScalingMetricSpec{
		Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
	}
	ir := newTestInterceptorRoute(scalingMetric)
	hdl := newTestScalerHandler(t, ir, aggregatedCount{Concurrency: 3})
	hdl.streamInterval = 50 * time.Millisecond

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(srv, hdl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dialing bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := externalscaler.NewExternalScalerClient(conn)
	streamCtx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()
	stream, err := client.StreamIsActive(streamCtx, testScaledObjectRef)
	if err != nil {
		t.Fatalf("StreamIsActive: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("receiving initial active event: %v", err)
	}
	if !resp.Result {
		t.Fatal("initial active event reported inactive")
	}

	key := k8s.ResourceKey(ir.Namespace, ir.Name)
	setCount := func(count aggregatedCount) {
		hdl.pinger.pingMut.Lock()
		defer hdl.pinger.pingMut.Unlock()
		hdl.pinger.allCounts[key] = count
	}

	setCount(aggregatedCount{})
	type streamResult struct {
		response *externalscaler.IsActiveResponse
		err      error
	}
	resultCh := make(chan streamResult, 1)
	go func() {
		response, err := stream.Recv()
		resultCh <- streamResult{response: response, err: err}
	}()

	select {
	case result := <-resultCh:
		t.Fatalf("received active event while inactive: response=%v, error=%v", result.response, result.err)
	case <-time.After(150 * time.Millisecond):
	}

	setCount(aggregatedCount{Concurrency: 3})
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("receiving reactivation event: %v", result.err)
		}
		if !result.response.Result {
			t.Fatal("reactivation event reported inactive")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for reactivation event")
	}

	_, err = stream.Recv()
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("expected duplicate active event to be suppressed, got error: %v", err)
	}
}

func TestStreamMetricSpec(t *testing.T) {
	t.Run("pushes the spec on open and on every change", func(t *testing.T) {
		ir := newTestInterceptorRoute(httpv1beta1.ScalingMetricSpec{
			Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 5},
		})
		hdl, cl := newTestScalerHandlerWithClient(t, ir, aggregatedCount{})

		client := newTestScalerClient(t, hdl)

		streamCtx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		stream, err := client.StreamMetricSpec(streamCtx, testScaledObjectRef)
		if err != nil {
			t.Fatalf("StreamMetricSpec: %v", err)
		}

		assertStreamedTarget(t, stream, testIRConcurrencyMetric, 5)

		// A notification that leaves the targets untouched must not be
		// forwarded, so the next received spec is the one set below.
		hdl.notifySpecChange(testIRKey)

		updateIRConcurrency(t, cl, 10)
		hdl.notifySpecChange(testIRKey)

		assertStreamedTarget(t, stream, testIRConcurrencyMetric, 10)
	})

	t.Run("ignores changes for another IR", func(t *testing.T) {
		ir := newTestInterceptorRoute(httpv1beta1.ScalingMetricSpec{
			Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 5},
		})
		hdl, cl := newTestScalerHandlerWithClient(t, ir, aggregatedCount{})

		client := newTestScalerClient(t, hdl)

		streamCtx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		stream, err := client.StreamMetricSpec(streamCtx, testScaledObjectRef)
		if err != nil {
			t.Fatalf("StreamMetricSpec: %v", err)
		}

		assertStreamedTarget(t, stream, testIRConcurrencyMetric, 5)

		updateIRConcurrency(t, cl, 10)
		hdl.notifySpecChange(types.NamespacedName{Namespace: testIRNamespace, Name: "another-ir"})

		if _, err := stream.Recv(); status.Code(err) != codes.DeadlineExceeded {
			t.Fatalf("expected unrelated change to be ignored, got error: %v", err)
		}
	})

	t.Run("IR not found", func(t *testing.T) {
		hdl, _ := newTestScalerHandlerWithClient(t, nil, aggregatedCount{})

		client := newTestScalerClient(t, hdl)

		streamCtx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		stream, err := client.StreamMetricSpec(streamCtx, testScaledObjectRef)
		if err != nil {
			t.Fatalf("StreamMetricSpec: %v", err)
		}

		if _, err := stream.Recv(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// newTestScalerClient serves the handler over an in-memory connection and
// returns a client for it.
func newTestScalerClient(t *testing.T, hdl *scalerHandler) externalscaler.ExternalScalerClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(srv, hdl)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dialing bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return externalscaler.NewExternalScalerClient(conn)
}

func updateIRConcurrency(t *testing.T, cl client.Client, target int32) {
	t.Helper()

	var ir httpv1beta1.InterceptorRoute
	nn := types.NamespacedName{Namespace: testIRNamespace, Name: testIRName}
	if err := cl.Get(t.Context(), nn, &ir); err != nil {
		t.Fatalf("getting InterceptorRoute: %v", err)
	}

	ir.Spec.ScalingMetric.Concurrency = &httpv1beta1.ConcurrencyTargetSpec{TargetValue: target}
	if err := cl.Update(t.Context(), &ir); err != nil {
		t.Fatalf("updating InterceptorRoute: %v", err)
	}
}

func assertStreamedTarget(t *testing.T, stream grpc.ServerStreamingClient[externalscaler.GetMetricSpecResponse], metricName string, target float64) {
	t.Helper()

	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream.Recv: %v", err)
	}

	specs := resp.GetMetricSpecs()
	if len(specs) != 1 {
		t.Fatalf("got %d metric specs, want 1", len(specs))
	}
	if got := specs[0].MetricName; got != metricName {
		t.Errorf("specs[0].MetricName = %q, want %q", got, metricName)
	}
	if got := specs[0].TargetSizeFloat; got != target {
		t.Errorf("specs[0].TargetSizeFloat = %v, want %v", got, target)
	}
}
