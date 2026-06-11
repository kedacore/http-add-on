//go:build e2e

package benchmark_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestThroughputWithManyRoutes(t *testing.T) {
	feat := features.New("throughput-with-many-routes").
		WithLabel("area", "benchmark").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx, _ = setupFixedReplicaApp(ctx, t, "many-routes")
			return ctx
		}).
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			app := f.CreateTestApp("route-filler-app", h.AppWithReplicas(1))

			t.Logf("creating %d filler InterceptorRoutes", benchCfg.RouteCount)
			for i := range benchCfg.RouteCount {
				f.CreateInterceptorRouteNoWait(
					fmt.Sprintf("filler-%d-ir", i),
					app,
					h.IRWithHosts(fmt.Sprintf("filler-%d.example.com", i)),
				)
			}
			time.Sleep(5 * time.Second)
			t.Logf("all %d filler routes created", benchCfg.RouteCount)

			return ctx
		}).
		Assess("throughput with many routes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			result := f.RunOha(h.OhaOpts{
				Rate:     benchCfg.RouteLoadRate,
				Duration: benchCfg.RouteLoadDuration,
				Host:     f.Hostname(),
			})
			f.LogOhaResult(result)
			assertThresholds(t, result)

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
