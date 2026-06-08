package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDrainHandler(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := map[string]struct {
		draining   bool
		protoMajor int
		wantHeader string
	}{
		"not draining": {
			protoMajor: 1,
			wantHeader: "",
		},
		"draining HTTP/1.1": {
			draining:   true,
			protoMajor: 1,
			wantHeader: "close",
		},
		"draining HTTP/2": {
			draining:   true,
			protoMajor: 2,
			wantHeader: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var draining atomic.Bool
			if tc.draining {
				draining.Store(true)
			}

			h := newDrainHandler(ok, &draining)
			req := httptest.NewRequest("GET", "/", nil)
			req.ProtoMajor = tc.protoMajor
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Connection"); got != tc.wantHeader {
				t.Errorf("Connection header = %q, want %q", got, tc.wantHeader)
			}
		})
	}
}
