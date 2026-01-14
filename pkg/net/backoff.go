package net

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// MinTotalBackoffDuration returns the minimum total sleep duration across all retry steps.
func MinTotalBackoffDuration(backoff wait.Backoff) time.Duration {
	var total time.Duration
	duration := backoff.Duration
	for range backoff.Steps {
		if backoff.Cap > 0 && duration > backoff.Cap {
			duration = backoff.Cap
		}
		total += duration
		duration = time.Duration(float64(duration) * backoff.Factor)
	}
	return total
}
