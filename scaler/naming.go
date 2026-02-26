package main

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/types"
)

var unsafeChars = regexp.MustCompile(`[^-.0-9A-Za-z]`)

func escapeRune(r string) string {
	return fmt.Sprintf("_%04X", r)
}

func escapeString(s string) string {
	return unsafeChars.ReplaceAllStringFunc(s, escapeRune)
}

func MetricName(namespacedName *types.NamespacedName) string {
	mn := fmt.Sprintf("http-%v", namespacedName)
	return escapeString(mn)
}
