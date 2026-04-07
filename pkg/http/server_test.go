package http

import (
	"crypto/tls"
	"net"
	"net/http"
	"testing"

	"github.com/kedacore/http-add-on/pkg/testutil"
)

func TestServe(t *testing.T) {
	cert, caPool := testutil.GenerateCert(t, []string{"localhost"}, []net.IP{net.IPv4(127, 0, 0, 1)})

	tests := map[string]struct {
		serverTLS *tls.Config
		clientTLS *tls.Config
	}{
		"plain HTTP": {},
		"TLS": {
			serverTLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
			clientTLS: &tls.Config{
				RootCAs: caPool,
			},
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
			hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			go func() {
				_ = serve(t.Context(), ln, hdl, tc.serverTLS)
			}()

			// Send a test request
			scheme := "http"
			if tc.clientTLS != nil {
				scheme = "https"
			}
			client := &http.Client{Transport: &http.Transport{TLSClientConfig: tc.clientTLS}}

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
		})
	}
}
