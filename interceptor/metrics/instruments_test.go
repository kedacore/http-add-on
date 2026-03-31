package metrics

import (
	"testing"
)

func TestInstruments_MethodNormalization(t *testing.T) {
	tests := map[string]string{
		"GET":     "GET",
		"POST":    "POST",
		"PUT":     "PUT",
		"DELETE":  "DELETE",
		"PATCH":   "PATCH",
		"HEAD":    "HEAD",
		"OPTIONS": "OPTIONS",
		"CONNECT": "CONNECT",
		"TRACE":   "TRACE",
		"PURGE":   "_OTHER",
		"":        "_OTHER",
		"get":     "_OTHER",
	}

	for input, want := range tests {
		got := normalizeMethod(input)
		if got != want {
			t.Errorf("normalizeMethod(%q) = %q, want %q", input, got, want)
		}
	}
}
