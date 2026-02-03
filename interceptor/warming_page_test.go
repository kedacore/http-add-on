package main

import (
	"strings"
	"testing"
)

func TestGenerateWarmingPageHTML(t *testing.T) {
	tests := []struct {
		name            string
		customMessage   string
		expectedContent []string
	}{
		{
			name:          "with custom message",
			customMessage: "Waking up the application...",
			expectedContent: []string{
				"Waking up the application...",
				"Service Starting",
				"meta http-equiv=\"refresh\"",
				"This page will automatically refresh",
			},
		},
		{
			name:          "with empty message uses default",
			customMessage: "",
			expectedContent: []string{
				defaultColdStartMessage,
				"Service Starting",
				"meta http-equiv=\"refresh\"",
			},
		},
		{
			name:          "escapes HTML in message",
			customMessage: "<script>alert('xss')</script>",
			expectedContent: []string{
				"&lt;script&gt;",
				"&lt;/script&gt;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := generateWarmingPageHTML(tt.customMessage)

			// Check that HTML is not empty
			if html == "" {
				t.Error("generated HTML is empty")
			}

			// Check for expected content
			for _, expected := range tt.expectedContent {
				if !strings.Contains(html, expected) {
					t.Errorf("HTML does not contain expected content: %q", expected)
				}
			}

			// Verify it's valid HTML structure
			if !strings.Contains(html, "<!DOCTYPE html>") {
				t.Error("HTML missing DOCTYPE declaration")
			}
			if !strings.Contains(html, "<html") {
				t.Error("HTML missing html tag")
			}
			if !strings.Contains(html, "</html>") {
				t.Error("HTML missing closing html tag")
			}
		})
	}
}

func TestGenerateWarmingPageHTML_XSSProtection(t *testing.T) {
	dangerousInputs := []struct {
		input          string
		shouldNotContain string
	}{
		{"<script>alert('xss')</script>", "<script>"},
		{"<img src=x onerror=alert('xss')>", "<img"},
		{"<iframe src='javascript:alert(1)'>", "<iframe"},
		{"<svg onload=alert('xss')>", "<svg"},
	}

	for _, tc := range dangerousInputs {
		t.Run("XSS_protection_"+tc.input, func(t *testing.T) {
			html := generateWarmingPageHTML(tc.input)

			// Verify dangerous tags are escaped (converted to &lt; &gt;)
			if strings.Contains(html, tc.shouldNotContain) {
				t.Errorf("HTML contains unescaped dangerous tag: %s", tc.shouldNotContain)
			}
			
			// Verify the escaped version is present
			if !strings.Contains(html, "&lt;") {
				t.Error("HTML should contain escaped angle brackets")
			}
		})
	}
}
