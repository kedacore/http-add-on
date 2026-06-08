//go:build e2e

package benchmark_test

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestThroughput(t *testing.T) {
	feat := features.New("throughput").
		WithLabel("area", "benchmark").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx, _ = setupFixedReplicaApp(ctx, t, "throughput")
			return ctx
		}).
		Assess("steady-state throughput", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			result := f.RunOha(h.OhaOpts{
				Rate:     benchCfg.ThroughputRate,
				Duration: benchCfg.ThroughputDuration,
				Host:     f.Hostname(),
			})
			f.LogOhaResult(result)
			assertThresholds(t, result)

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestUncappedThroughput(t *testing.T) {
	feat := features.New("uncapped-throughput").
		WithLabel("area", "benchmark").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ctx, _ = setupFixedReplicaApp(ctx, t, "uncapped")
			return ctx
		}).
		Assess("uncapped request rate", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			result := f.RunOha(h.OhaOpts{
				Rate:     0, // uncapped
				Duration: benchCfg.UncappedDuration,
				Host:     f.Hostname(),
			})
			f.LogOhaResult(result)

			rps := result.Summary.RequestsPerSec
			t.Logf("achieved_rps=%.1f (min %.1f)", rps, benchCfg.UncappedMinRPS)
			assertThresholds(t, result)
			if rps < benchCfg.UncappedMinRPS {
				t.Errorf("achieved rps %.1f below minimum %.1f", rps, benchCfg.UncappedMinRPS)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
