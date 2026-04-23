package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	testScaledObjectRef     = &externalscaler.ScaledObjectRef{
		Name:      "test-so",
		Namespace: testIRNamespace,
		ScalerMetadata: map[string]string{
			k8s.InterceptorRouteKey: testIRName,
		},
	}
)

// newFakeClient creates a fake client populated with the given objects.
func newFakeClient(objs ...runtime.Object) client.Reader {
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

	var reader client.Reader
	if ir != nil {
		reader = newFakeClient(ir)
	} else {
		reader = newFakeClient()
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

	return newScalerHandler(logr.Discard(), pinger, reader, time.Second)
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
	tests := map[string]struct {
		count      aggregatedCount
		wantActive bool
	}{
		"active": {
			count:      aggregatedCount{Concurrency: 3},
			wantActive: true,
		},
		"inactive": {
			wantActive: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scalingMetric := httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: 100},
			}
			ir := newTestInterceptorRoute(scalingMetric)
			hdl := newTestScalerHandler(t, ir, tc.count)
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

			stream, err := client.StreamIsActive(t.Context(), testScaledObjectRef)
			if err != nil {
				t.Fatalf("StreamIsActive: %v", err)
			}

			resp, err := stream.Recv()
			if err != nil {
				t.Fatalf("stream.Recv: %v", err)
			}

			if got, want := resp.Result, tc.wantActive; got != want {
				t.Errorf("StreamIsActive result = %v, want %v", got, want)
			}
		})
	}
}
