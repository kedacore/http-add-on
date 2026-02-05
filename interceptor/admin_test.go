package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
)

func TestAdminHandler(t *testing.T) {
	tests := map[string]struct {
		path            string
		wantProbeCalled bool
		wantStatus      int
	}{
		"livez": {
			path:            "/livez",
			wantProbeCalled: true,
			wantStatus:      http.StatusOK,
		},
		"readyz": {
			path:            "/readyz",
			wantProbeCalled: true,
			wantStatus:      http.StatusOK,
		},
		"other": {
			path:            "/other",
			wantProbeCalled: false,
			wantStatus:      http.StatusNotFound,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			probeCalled := false
			probeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				probeCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := BuildAdminHandler(logr.Discard(), nil, probeHandler)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if probeCalled != tt.wantProbeCalled {
				t.Error("expected probe to be called")
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("got status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
