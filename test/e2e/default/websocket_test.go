//go:build e2e

package default_test

import (
	"context"
	"testing"

	"github.com/gorilla/websocket"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestWebSocket(t *testing.T) {
	t.Parallel()

	feat := features.New("websocket").
		WithLabel("area", "websocket").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("ws-app")
			ir := f.CreateInterceptorRoute("ws-ir", app,
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("ws-so", app, ir)

			return ctx
		}).
		Assess("upgrades to WebSocket and echoes messages", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			conn := f.WebSocketDial(f.Hostname(), "/echo")

			msg := "hello from e2e"
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				t.Fatalf("failed to write WebSocket message: %v", err)
			}

			_, received, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read WebSocket message: %v", err)
			}
			if string(received) != msg {
				t.Errorf("expected echo %q, got %q", msg, string(received))
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
