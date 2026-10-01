//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

const (
	sessionAppName    = "session-app"
	sessionCookieName = "e2e-session"
	sessionReplicas   = 3
)

func TestSessionPersistence(t *testing.T) {
	t.Parallel()

	feat := features.New("session-persistence").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp(sessionAppName, h.AppWithReplicas(sessionReplicas))
			f.CreateInterceptorRoute("session-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithSessionPersistence(httpv1beta1.SessionPersistence{
					Type:   httpv1beta1.SessionPersistenceTypeCookie,
					Cookie: httpv1beta1.SessionCookie{Name: sessionCookieName},
				}),
			)
			f.WaitForReplicas(app, sessionReplicas)

			// Wait until the interceptor routes to every pod, so pinning is
			// observable rather than incidental.
			seen := make(map[string]bool)
			waitFor(t, "requests to reach all pods", func() bool {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})
				if resp.StatusCode == http.StatusOK {
					seen[servingPod(ctx, t, cfg, f.Namespace(), resp.Body)] = true
				}
				return len(seen) == sessionReplicas
			})

			return ctx
		}).
		Assess("pins a session to one pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			first := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})
			cookie := requireSessionCookie(t, first)
			pinned := servingPod(ctx, t, cfg, f.Namespace(), first.Body)

			for range 20 {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname(), Headers: map[string]string{"Cookie": cookie}})
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
				}
				if got := servingPod(ctx, t, cfg, f.Namespace(), resp.Body); got != pinned {
					t.Fatalf("request served by %q, want pinned pod %q", got, pinned)
				}
				if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
					t.Fatalf("expected no Set-Cookie for an established session, got %q", got)
				}
			}

			return ctx
		}).
		Assess("reassigns a session when its pod is deleted", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			first := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})
			cookie := requireSessionCookie(t, first)
			pinned := servingPod(ctx, t, cfg, f.Namespace(), first.Body)

			pod := &corev1.Pod{}
			if err := cfg.Client().Resources(f.Namespace()).Get(ctx, pinned, f.Namespace(), pod); err != nil {
				t.Fatalf("failed to get pod %s: %v", pinned, err)
			}
			f.DeleteResource(pod)

			// The interceptor keeps the old pod until it observes the
			// EndpointSlice update, so poll for the reassignment.
			var reassigned string
			waitFor(t, "session to be reassigned", func() bool {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname(), Headers: map[string]string{"Cookie": cookie}})
				if resp.StatusCode != http.StatusOK || servingPod(ctx, t, cfg, f.Namespace(), resp.Body) == pinned {
					return false
				}
				reassigned = requireSessionCookie(t, resp)
				return true
			})

			newPinned := ""
			for range 10 {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname(), Headers: map[string]string{"Cookie": reassigned}})
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
				}
				got := servingPod(ctx, t, cfg, f.Namespace(), resp.Body)
				if newPinned == "" {
					newPinned = got
				}
				if got != newPinned {
					t.Fatalf("request served by %q, want re-pinned pod %q", got, newPinned)
				}
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// requireSessionCookie returns the request Cookie header value for the session
// cookie set on resp.
func requireSessionCookie(t *testing.T, resp h.HTTPResponse) string {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
	}
	for _, c := range (&http.Response{Header: resp.Header}).Cookies() {
		if c.Name == sessionCookieName {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatalf("expected Set-Cookie %q, got %q", sessionCookieName, resp.Header.Values("Set-Cookie"))
	return ""
}

// servingPod returns the name of the session app pod whose IP appears in the
// whoami response body. All pods share the same hostname, so the pod IP
// identifies the pod.
func servingPod(ctx context.Context, t *testing.T, cfg *envconf.Config, namespace string, body []byte) string {
	t.Helper()

	var pods corev1.PodList
	selector := resources.WithLabelSelector(labels.Set{"app": sessionAppName}.String())
	if err := cfg.Client().Resources(namespace).List(ctx, &pods, selector); err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}
	for _, pod := range pods.Items {
		for line := range strings.Lines(string(body)) {
			if pod.Status.PodIP != "" && strings.TrimSpace(line) == "IP: "+pod.Status.PodIP {
				return pod.Name
			}
		}
	}
	t.Fatalf("no %s pod matches the response; body: %s", sessionAppName, body)
	return ""
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Minute)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
