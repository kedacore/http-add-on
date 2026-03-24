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

func MetricNameHTTPSO(namespacedName *types.NamespacedName) string {
	// TODO(v1): remove this func and the ones above with the removal of HTTPSO
	mn := fmt.Sprintf("http-%v", namespacedName)
	return escapeString(mn)
}

func ConcurrencyMetricName(irName string) string {
	return metricName(irName, "concurrency")
}

func RateMetricName(irName string) string {
	return metricName(irName, "rate")
}

func metricName(irName string, metricType string) string {
	return fmt.Sprintf("http_%s_%s", irName, metricType)
}
