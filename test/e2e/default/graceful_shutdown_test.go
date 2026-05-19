//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestGracefulShutdown(t *testing.T) {
	// Not parallel: this test restarts the interceptor pod, which would
	// disrupt any other tests running concurrently.

	feat := features.New("graceful-shutdown").
		WithLabel("area", "reliability").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("shutdown-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("shutdown-ir", app,
				h.IRWithHosts(f.Hostname()),
			)

			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "shutdown-app")
			return ctx
		}).
		Assess("in-flight request survives interceptor restart", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			restartErrCh := make(chan error, 1)
			go func() {
				// Give the request time to reach the backend and start waiting
				time.Sleep(2 * time.Second)
				restartErrCh <- f.RestartInterceptor()
			}()

			// Send a slow request that takes 10s to complete. If the interceptor
			// kills in-flight requests during shutdown, this will fail with a
			// connection error. The ProxyRequestRaw helper will call t.Fatalf,
			// which is exactly what we want - a clear test failure.
			resp := f.ProxyRequestRaw(h.Request{
				Host:  f.Hostname(),
				Query: "wait=10s",
			})

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, string(resp.Body))
			}

			if err := <-restartErrCh; err != nil {
				t.Fatalf("failed to restart interceptor: %v", err)
			}
			return ctx
		}).
		Assess("requests work after restart", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "shutdown-app")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
