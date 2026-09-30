package main

import "fmt"

func ConcurrencyMetricName(irName string) string {
	return metricName(irName, "concurrency")
}

func RateMetricName(irName string) string {
	return metricName(irName, "rate")
}

func metricName(irName string, metricType string) string {
	return fmt.Sprintf("http_%s_%s", irName, metricType)
}
