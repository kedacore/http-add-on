package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/testutil"
)

func TestServe(t *testing.T) {
	cert, caPool := testutil.GenerateCert(t, []string{"localhost"}, []net.IP{net.IPv4(127, 0, 0, 1)})

	var http2Proto http.Protocols
	http2Proto.SetUnencryptedHTTP2(true)
	http2Proto.SetHTTP2(true)

	tests := map[string]struct {
		serverTLS   *tls.Config
		clientTLS   *tls.Config
		clientProto *http.Protocols
		wantProto   int
	}{
		"plain HTTP": {
			wantProto: 1,
		},
		"h2c": {
			clientProto: &http2Proto,
			wantProto:   2,
		},
		"h2 over TLS": {
			serverTLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
			clientTLS: &tls.Config{
				RootCAs: caPool,
			},
			clientProto: &http2Proto,
			wantProto:   2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a listener on a free port
			ln, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}

			// Launch the server
			var serverProtoMajor int
			hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverProtoMajor = r.ProtoMajor
				w.WriteHeader(http.StatusOK)
			})
			go func() {
				_ = serve(t.Context(), ln, ServerConfig{
					Handler:   hdl,
					TLSConfig: tc.serverTLS,
				})
			}()

			// Send a test request
			scheme := "http"
			if tc.clientTLS != nil {
				scheme = "https"
			}
			client := &http.Client{Transport: &http.Transport{
				TLSClientConfig: tc.clientTLS,
				Protocols:       tc.clientProto,
			}}

			resp, err := client.Get(scheme + "://" + ln.Addr().String())
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			_ = resp.Body.Close()

			// Verify response
			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
			if tc.serverTLS != nil && resp.TLS == nil {
				t.Fatal("expected TLS connection, but resp.TLS is nil")
			}
			if serverProtoMajor != tc.wantProto {
				t.Fatalf("server saw HTTP/%d, want HTTP/%d", serverProtoMajor, tc.wantProto)
			}
		})
	}
}

func TestServeWaitsForInFlightRequests(t *testing.T) {
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})

	hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		<-handlerDone
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveReturned := make(chan error, 1)
	go func() {
		serveReturned <- serve(ctx, ln, ServerConfig{Handler: hdl})
	}()

	// Send a request that will block in the handler.
	var resp *http.Response
	var reqErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		resp, reqErr = http.Get("http://" + ln.Addr().String()) //nolint:bodyclose // closed below after wg.Wait
	})

	// Wait for the handler to be processing the request.
	<-handlerStarted

	// Cancel the context (simulates SIGTERM). This should trigger Shutdown()
	// which closes the listener, but serve() must NOT return until the
	// in-flight handler finishes.
	cancel()

	// Give serve() a moment to (incorrectly) return early.
	select {
	case <-serveReturned:
		t.Fatal("serve() returned while a handler was still in-flight — the shutdown drain is broken")
	case <-time.After(200 * time.Millisecond):
	}

	// Let the handler finish.
	close(handlerDone)

	// Now serve() should return.
	select {
	case err := <-serveReturned:
		if err != nil {
			t.Fatalf("serve() returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after in-flight handler completed")
	}

	// The HTTP request should have succeeded.
	wg.Wait()
	if reqErr != nil {
		t.Fatalf("HTTP request failed: %v", reqErr)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestServeWaitsForHijackedConnections(t *testing.T) {
	handlerStarted := make(chan struct{})
	ctxCancelled := make(chan struct{})
	handlerGate := make(chan struct{})

	hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(fmt.Sprintf("hijack failed: %v", err))
		}
		defer conn.Close()

		close(handlerStarted)

		// Wait for context cancellation, then signal it happened.
		<-r.Context().Done()
		close(ctxCancelled)

		// Simulate cleanup work after context cancellation.
		<-handlerGate
	})

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveReturned := make(chan error, 1)
	go func() {
		serveReturned <- serve(ctx, ln, ServerConfig{Handler: hdl})
	}()

	// Send a raw HTTP request that will be hijacked.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	<-handlerStarted

	// Cancel context (SIGTERM).
	cancel()

	// The handler's request context should be cancelled.
	select {
	case <-ctxCancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("hijacked handler was not notified of shutdown via context cancellation")
	}

	// serve() must NOT return while the hijacked handler is still running.
	select {
	case <-serveReturned:
		t.Fatal("serve() returned while a hijacked handler was still in-flight")
	case <-time.After(200 * time.Millisecond):
	}

	// Let the handler finish.
	close(handlerGate)

	// Now serve() should return.
	select {
	case err := <-serveReturned:
		if err != nil {
			t.Fatalf("serve() returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after hijacked handler completed")
	}
}

func TestServeDrainTimeout(t *testing.T) {
	handlerStarted := make(chan struct{})

	// A handler that blocks forever (simulates a hung request).
	hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		select {} // block forever
	})

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	drainTimeout := 200 * time.Millisecond

	serveReturned := make(chan error, 1)
	go func() {
		serveReturned <- serve(ctx, ln, ServerConfig{
			Handler:      hdl,
			DrainTimeout: drainTimeout,
		})
	}()

	// Fire off a request that will hang.
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-handlerStarted

	// Cancel context (SIGTERM).
	cancel()

	// serve() should return around drainTimeout, not hang forever.
	select {
	case err := <-serveReturned:
		if err != nil {
			t.Fatalf("serve() returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve() did not return after drain timeout — it hung forever")
	}
}
