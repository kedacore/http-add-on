package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstrumentedResponseWriter_Unwrap(t *testing.T) {
	w := httptest.NewRecorder()
	rw := newInstrumentedResponseWriter(w)

	if got := rw.Unwrap(); got != w {
		t.Fatalf("got %v, want %v", got, w)
	}
}

func TestInstrumentedResponseWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	rw := newInstrumentedResponseWriter(w)

	if _, err := rw.Write([]byte("te")); err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Write([]byte("st")); err != nil {
		t.Fatal(err)
	}

	if got, want := rw.bytesWritten, 4; got != want {
		t.Fatalf("bytesWritten: got %d, want %d", got, want)
	}
	if got, want := w.Body.String(), "test"; got != want {
		t.Fatalf("body: got %q, want %q", got, want)
	}
}

func TestInstrumentedResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := newInstrumentedResponseWriter(w)

	if got, want := rw.statusCode, 0; got != want {
		t.Fatalf("statusCode before WriteHeader: got %d, want %d", got, want)
	}

	rw.WriteHeader(http.StatusTeapot)

	if got, want := rw.statusCode, http.StatusTeapot; got != want {
		t.Fatalf("statusCode: got %d, want %d", got, want)
	}
	if got, want := w.Code, http.StatusTeapot; got != want {
		t.Fatalf("downstream code: got %d, want %d", got, want)
	}
}
