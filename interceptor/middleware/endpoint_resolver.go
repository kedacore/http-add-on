package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/kedacore/http-add-on/interceptor/handler"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const defaultFallbackReadinessTimeout = 30 * time.Second

type EndpointResolverConfig struct {
	ReadinessTimeout      time.Duration
	EnableColdStartHeader bool
}

type EndpointResolver struct {
	next       http.Handler
	readyCache *k8s.ReadyEndpointsCache
	cfg        EndpointResolverConfig
}

// NewEndpointResolver returns a middleware that resolves a ready backend
// endpoint for each request. It waits for at least one endpoint to become
// ready (handling cold starts) and optionally falls back to an alternate
// upstream when the backend does not become ready in time.
func NewEndpointResolver(next http.Handler, readyCache *k8s.ReadyEndpointsCache, cfg EndpointResolverConfig) *EndpointResolver {
	return &EndpointResolver{
		next:       next,
		readyCache: readyCache,
		cfg:        cfg,
	}
}

var _ http.Handler = (*EndpointResolver)(nil)

func (er *EndpointResolver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ir := util.InterceptorRouteFromContext(ctx)

	readinessTimeout := er.cfg.ReadinessTimeout
	// Per-route override from InterceptorRoute spec
	if ir.Spec.Timeouts.Readiness != nil {
		readinessTimeout = ir.Spec.Timeouts.Readiness.Duration
	}

	hasFallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil
	// Bound the readiness wait or otherwise there is no time for the fallback
	if hasFallback && readinessTimeout == 0 {
		readinessTimeout = defaultFallbackReadinessTimeout
	}

	waitCtx := ctx
	if readinessTimeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, readinessTimeout)
		defer cancel()
	}

	serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service

	// Streaming callback: if the route has a StreamingCallback configured and
	// the backend is not ready, check if this is a streaming request. If so,
	// send SSE keepalive events while waiting for the backend.
	hasStreamingCallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.StreamingCallback != nil
	if hasStreamingCallback && !er.readyCache.HasReadyEndpoints(serviceKey) {
		streaming, err := isStreamingRequest(r)
		if err != nil {
			util.LoggerFromContext(ctx).Error(err, "failed to check streaming request")
		}
		if err == nil && streaming {
			er.serveStreamingCallback(waitCtx, ctx, w, r, serviceKey, ir)
			return
		}
	}

	isColdStart, err := er.readyCache.WaitForReady(waitCtx, serviceKey)
	if err != nil {
		// No fallback, return an error
		if !hasFallback {
			code := http.StatusBadGateway
			// Context expired or aborted — no time remaining to reach the backend.
			if waitCtx.Err() != nil {
				code = http.StatusGatewayTimeout
			}
			handler.
				NewStatic(code, fmt.Errorf("backend not ready: %w", err)).
				ServeHTTP(w, r)
			return
		}

		// Has fallback but parent context expired, error early
		if ctx.Err() != nil {
			handler.
				NewStatic(http.StatusGatewayTimeout, fmt.Errorf("backend not ready and no time remaining for fallback: %w", err)).
				ServeHTTP(w, r)
			return
		}

		// Fall back to alternate upstream.
		fallbackURL := util.FallbackURLFromContext(ctx)
		ctx = util.ContextWithUpstreamURL(ctx, fallbackURL)
		r = r.WithContext(ctx)
	}

	// isColdStart is only meaningful when the backend resolved without errors
	if err == nil && er.cfg.EnableColdStartHeader {
		w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
	}

	er.next.ServeHTTP(w, r)
}

// serveStreamingCallback handles cold-start waits for streaming requests by
// sending OpenAI-compatible SSE keepalive events until the backend is ready.
func (er *EndpointResolver) serveStreamingCallback(
	waitCtx, parentCtx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	serviceKey string,
	ir *httpv1beta1.InterceptorRoute,
) {
	logger := util.LoggerFromContext(parentCtx)
	cb := ir.Spec.ColdStart.StreamingCallback

	rc := http.NewResponseController(w)

	// Write SSE headers — commits to a 200 response.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Send initial loading message.
	if err := writeSSEEvent(w, cb.Message); err != nil {
		logger.Error(err, "failed to write initial streaming callback event")
		return
	}
	if err := rc.Flush(); err != nil {
		logger.Error(err, "failed to flush initial streaming callback event")
		return
	}

	interval := cb.Interval.Duration
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// Start keepalive goroutine.
	callbackDone := make(chan struct{})
	callbackStopped := make(chan struct{})
	go func() {
		defer close(callbackStopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := writeSSEEvent(w, cb.KeepaliveMessage); err != nil {
					logger.Error(err, "failed to write keepalive streaming callback event")
					return
				}
				if err := rc.Flush(); err != nil {
					logger.Error(err, "failed to flush keepalive streaming callback event")
					return
				}
			case <-callbackDone:
				return
			}
		}
	}()

	// Wait for the backend to become ready.
	isColdStart, err := er.readyCache.WaitForReady(waitCtx, serviceKey)
	close(callbackDone)
	<-callbackStopped // ensure goroutine exits before touching w again

	if err != nil {
		// Already committed to 200, so send an SSE error event instead of HTTP error.
		logger.Error(err, "backend not ready during streaming callback")
		errMsg := fmt.Sprintf("Backend did not become ready: %v", err)
		_ = writeSSEEvent(w, errMsg)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		_ = rc.Flush()
		return
	}

	if er.cfg.EnableColdStartHeader {
		w.Header().Set(kedahttp.HeaderColdStart, strconv.FormatBool(isColdStart))
	}

	// Send a visual separator so the real model response starts on a fresh
	// line, rather than being appended directly after the keepalive dots.
	// The content ends with a zero-width space (U+200B) after the final
	// newline because common shell-based clients use bash command
	// substitution which strips trailing newlines. The zero-width space
	// anchors the newline so it survives extraction while remaining
	// invisible in the terminal output.
	if err := writeSSEEvent(w, "\n\n---\n\u200B"); err != nil {
		logger.Error(err, "failed to write separator streaming callback event")
	} else {
		_ = rc.Flush()
	}

	// Wrap the writer to suppress duplicate WriteHeader from the upstream proxy.
	er.next.ServeHTTP(&headerSuppressingWriter{
		ResponseWriter: w,
		headerWritten:  true,
	}, r)
}

// isStreamingRequest checks whether the JSON request body contains "stream": true.
// It restores the body so subsequent handlers can re-read it.
func isStreamingRequest(r *http.Request) (bool, error) {
	if r.Body == nil {
		return false, nil
	}
	body, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	if len(body) == 0 {
		return false, nil
	}
	var payload struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, nil //nolint:nilerr // non-JSON body is not an error
	}
	return payload.Stream, nil
}

// writeSSEEvent writes a single OpenAI-compatible chat.completion.chunk SSE event.
func writeSSEEvent(w http.ResponseWriter, content string) error {
	contentJSON, _ := json.Marshal(content)
	_, err := fmt.Fprintf(w,
		"data: {\"id\":\"keda-cold-start\",\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":\"system\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%s},\"finish_reason\":null}]}\n\n",
		time.Now().Unix(),
		contentJSON,
	)
	return err
}

// headerSuppressingWriter wraps a ResponseWriter and silently ignores
// WriteHeader calls after the first one. This prevents the upstream
// reverse proxy from logging "superfluous response.WriteHeader call"
// when we have already committed to a 200 for SSE streaming.
type headerSuppressingWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *headerSuppressingWriter) WriteHeader(code int) {
	if w.headerWritten {
		return
	}
	w.headerWritten = true
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter so that
// http.NewResponseController can find optional interfaces (Flusher, Hijacker, etc.).
func (w *headerSuppressingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
