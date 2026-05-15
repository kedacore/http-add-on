package main

import (
	"regexp"
	"testing"
)

var safePattern = regexp.MustCompile(`^[-.0-9A-Za-z_]*$`)

// FuzzEscapeString verifies that escapeString always produces output containing
// only metric-safe characters ([-.0-9A-Za-z_]), regardless of the input.
func FuzzEscapeString(f *testing.F) {
	f.Add("simple")
	f.Add("with spaces")
	f.Add("with/slashes")
	f.Add("")
	f.Add("unicode-\u00e9\u00e8\u00ea")
	f.Add("emoji-\U0001f600")
	f.Add("null\x00byte")
	f.Add(string(make([]byte, 256)))

	f.Fuzz(func(t *testing.T, s string) {
		result := escapeString(s)
		if !safePattern.MatchString(result) {
			t.Errorf("escapeString(%q) = %q, contains unsafe characters", s, result)
		}
	})
}
