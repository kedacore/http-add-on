package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

func TestHTTP2H2CSupport(t *testing.T) {
	// Create a simple handler that reports the protocol version
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Protocol: %s", r.Proto)
	})

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverAddr := "127.0.0.1:18888"
	serverErrors := make(chan error, 1)

	go func() {
		if err := ServeContext(ctx, serverAddr, handler, nil); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Check for server startup errors
	select {
	case err := <-serverErrors:
		t.Fatalf("Server failed to start: %v", err)
	default:
	}

	t.Run("HTTP/1.1 should work", func(t *testing.T) {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		resp, err := client.Get(fmt.Sprintf("http://%s/test", serverAddr))
		if err != nil {
			t.Fatalf("HTTP/1.1 request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("HTTP/1.1 Response: %s", string(body))

		if resp.Proto != "HTTP/1.1" {
			t.Errorf("Expected HTTP/1.1, got %s", resp.Proto)
		}
	})

	t.Run("HTTP/2 h2c should work", func(t *testing.T) {
		// Create HTTP/2 client with prior knowledge (h2c)
		client := &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					// Use standard dialer for h2c (cleartext HTTP/2)
					return net.Dial(network, addr)
				},
			},
			Timeout: 5 * time.Second,
		}

		resp, err := client.Get(fmt.Sprintf("http://%s/test-h2", serverAddr))
		if err != nil {
			t.Fatalf("HTTP/2 h2c request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("HTTP/2 Response: %s", string(body))

		if resp.Proto != "HTTP/2.0" {
			t.Errorf("Expected HTTP/2.0, got %s", resp.Proto)
		}
	})
}
