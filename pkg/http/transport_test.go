package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestTransportOutboundProtocol(t *testing.T) {
	var backendProtoMajor int
	srv := httptest.NewUnstartedServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		backendProtoMajor = r.ProtoMajor
	}))

	// Accept all protocols, we are only interesting in the outgoing protocols
	var protocols nethttp.Protocols
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &protocols

	srv.Start()
	defer srv.Close()

	defaultTransport, http2OnlyTransport := NewTransports(nethttp.DefaultTransport.(*nethttp.Transport).Clone())

	tests := map[string]struct {
		transport          *nethttp.Transport
		expectedProtoMajor int
	}{
		"default transport uses HTTP/1.1 for plain HTTP": {
			transport:          defaultTransport,
			expectedProtoMajor: 1,
		},
		"http2 transport uses h2c for plain HTTP": {
			transport:          http2OnlyTransport,
			expectedProtoMajor: 2,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(nethttp.MethodGet, srv.URL+"/test", nil)
			resp, err := tt.transport.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip failed: %v", err)
			}
			_ = resp.Body.Close()

			if backendProtoMajor != tt.expectedProtoMajor {
				t.Errorf("backend saw HTTP/%d, want HTTP/%d", backendProtoMajor, tt.expectedProtoMajor)
			}
		})
	}
}

func TestHTTP2OnlyTransportRejectsHTTP1Backend(t *testing.T) {
	_, http2OnlyTransport := NewTransports(nethttp.DefaultTransport.(*nethttp.Transport).Clone())

	// Create a test server that only supports HTTP/1
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {}))
	defer srv.Close()

	req := httptest.NewRequest(nethttp.MethodGet, srv.URL+"/test", nil)
	resp, err := http2OnlyTransport.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("http2Only transport should not connect to an HTTP/1-only backend")
	}
}
