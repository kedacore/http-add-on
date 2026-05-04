package main

import (
	"regexp"
	"strings"
	"testing"
)

var safePattern = regexp.MustCompile(`^[-.0-9A-Za-z_]*$`)

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

		result2 := escapeString(s)
		if result != result2 {
			t.Errorf("escapeString is not deterministic: %q != %q", result, result2)
		}
	})
}

func FuzzConcurrencyMetricName(f *testing.F) {
	f.Add("my-app")
	f.Add("")
	f.Add("name with spaces")
	f.Add("unicode-\u00e9")
	f.Add("emoji-\U0001f600")
	f.Add("a/b")

	f.Fuzz(func(t *testing.T, irName string) {
		result := ConcurrencyMetricName(irName)
		if !strings.HasPrefix(result, "http_") || !strings.HasSuffix(result, "_concurrency") {
			t.Errorf("ConcurrencyMetricName(%q) = %q, unexpected format", irName, result)
		}
	})
}

func FuzzRateMetricName(f *testing.F) {
	f.Add("my-app")
	f.Add("")
	f.Add("name with spaces")
	f.Add("unicode-\u00e9")
	f.Add("emoji-\U0001f600")
	f.Add("a/b")

	f.Fuzz(func(t *testing.T, irName string) {
		result := RateMetricName(irName)
		if !strings.HasPrefix(result, "http_") || !strings.HasSuffix(result, "_rate") {
			t.Errorf("RateMetricName(%q) = %q, unexpected format", irName, result)
		}
	})
}
