//go:build e2e

package benchmark_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	envparser "github.com/caarlos0/env/v11"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/pkg/env"

	h "github.com/kedacore/http-add-on/test/helpers"
)

type benchmarkConfig struct {
	Enabled bool `env:"BENCHMARK" envDefault:"false"`

	// Shared thresholds
	ErrorRateMax float64       `env:"BENCH_ERROR_RATE_MAX" envDefault:"0"`
	P99Max       time.Duration `env:"BENCH_P99_MAX" envDefault:"100ms"`

	// Cold start
	ColdStartSamples int           `env:"BENCH_COLD_START_SAMPLES" envDefault:"10"`
	ColdStartMax     time.Duration `env:"BENCH_COLD_START_MAX" envDefault:"3s"`

	// Throughput
	ThroughputRate     int           `env:"BENCH_THROUGHPUT_RATE" envDefault:"1000"`
	ThroughputDuration time.Duration `env:"BENCH_THROUGHPUT_DURATION" envDefault:"60s"`
	UncappedDuration   time.Duration `env:"BENCH_UNCAPPED_DURATION" envDefault:"60s"`
	UncappedMinRPS     float64       `env:"BENCH_UNCAPPED_MIN_RPS" envDefault:"3000"`

	// Preloaded tests
	RouteCount        int           `env:"BENCH_ROUTE_COUNT" envDefault:"200"`
	RouteLoadDuration time.Duration `env:"BENCH_ROUTE_LOAD_DURATION" envDefault:"30s"`
	RouteLoadRate     int           `env:"BENCH_ROUTE_LOAD_RATE" envDefault:"1000"`
}

var (
	benchCfg benchmarkConfig
	testenv  env.Environment
)

func TestMain(m *testing.M) {
	benchCfg = envparser.Must(envparser.ParseAs[benchmarkConfig]())
	if !benchCfg.Enabled {
		fmt.Println("Skipping benchmark tests (set BENCHMARK=true to run)")
		os.Exit(0)
	}
	testenv = h.NewTestEnv()
	os.Exit(testenv.Run(m))
}

// setupFixedReplicaApp creates a test app with a single fixed replica
// (no autoscaling) and waits for it to be ready.
func setupFixedReplicaApp(ctx context.Context, t *testing.T, name string) (context.Context, *h.TestApp) {
	t.Helper()
	f := h.NewFramework(ctx, t)

	app := f.CreateTestApp(name+"-app", h.AppWithReplicas(1))
	ir := f.CreateInterceptorRoute(name+"-ir", app,
		h.IRWithHosts(f.Hostname()),
	)
	f.CreateScaledObject(name+"-so", app, ir, func(so *kedav1alpha1.ScaledObject) {
		so.Spec.MinReplicaCount = ptr.To(int32(1))
		so.Spec.MaxReplicaCount = ptr.To(int32(1))
		so.Spec.IdleReplicaCount = nil
	})

	f.WaitForReplicas(app, 1)

	return ctx, app
}

// assertThresholds checks P99 latency, error rate, and status codes against the shared thresholds.
func assertThresholds(t *testing.T, result h.OhaResult) {
	t.Helper()

	p99 := time.Duration(result.LatencyPercentiles.P99 * float64(time.Second))
	errorRate := result.ErrorRate()
	t.Logf("p99=%v (max %v), error_rate=%.2f%% (max %.2f%%)",
		p99, benchCfg.P99Max, errorRate*100, benchCfg.ErrorRateMax*100)

	if p99 > benchCfg.P99Max {
		t.Errorf("p99 latency %v exceeds threshold %v", p99, benchCfg.P99Max)
	}
	if errorRate > benchCfg.ErrorRateMax {
		t.Errorf("error rate %.2f%% exceeds threshold %.2f%%", errorRate*100, benchCfg.ErrorRateMax*100)
	}
	for code, count := range result.StatusCodeDist {
		if code != "200" {
			t.Errorf("unexpected status code %s: %d requests", code, count)
		}
	}
}
