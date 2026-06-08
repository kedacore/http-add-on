//go:build e2e

package benchmark_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestScaleFromZeroLatency(t *testing.T) {
	var app *h.TestApp
	var host string

	feat := features.New("scale-from-zero-latency").
		WithLabel("area", "benchmark").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("cold-start-bench-app")
			host = f.Hostname()

			return ctx
		}).
		Assess("cold-start response times", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			n := benchCfg.ColdStartSamples
			durations := make([]time.Duration, 0, n)

			for i := range n {
				// Make sure the deployment is scaled down after the first test
				if i > 0 {
					f.ScaleDeployment(app, 0)
				}

				// Create a fresh ScaledObject so KEDA scales from zero on the next request
				name := fmt.Sprintf("cold-start-%d", i)
				ir := f.CreateInterceptorRoute(name+"-ir", app,
					h.IRWithHosts(host),
				)
				so := f.CreateScaledObject(name+"-so", app, ir)
				f.WaitForReplicas(app, 0)

				start := time.Now()
				resp := f.ProxyRequest(h.Request{Host: host})
				d := time.Since(start)

				if resp.StatusCode != http.StatusOK {
					t.Fatalf("iteration %d: expected status 200, got %d; body: %s",
						i, resp.StatusCode, string(resp.Body))
				}

				t.Logf("iteration %d: %v", i, d.Truncate(time.Millisecond))
				durations = append(durations, d)

				// Clean up before next iteration
				f.DeleteResource(so)
				f.DeleteResource(ir)
			}

			var total, maxDuration time.Duration
			for _, d := range durations {
				total += d
				if d > maxDuration {
					maxDuration = d
				}
			}
			avg := total / time.Duration(n)

			t.Logf("cold-start avg=%v max=%v (threshold %v) samples=%d",
				avg.Truncate(time.Millisecond), maxDuration.Truncate(time.Millisecond),
				benchCfg.ColdStartMax, n)

			if avg > benchCfg.ColdStartMax {
				t.Errorf("average cold-start %v exceeds threshold %v", avg, benchCfg.ColdStartMax)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
