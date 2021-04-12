package net

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// MinTotalBackoffDuration returns the minimum duration that backoff
// would wait, including all steps, not including any jitter
func MinTotalBackoffDuration(backoff wait.Backoff) time.Duration {
	initial := backoff.Duration.Milliseconds()
	retMS := backoff.Duration.Milliseconds()
	numSteps := backoff.Steps
	for i := 2; i <= numSteps; i++ {
		retMS += int64(initial) * int64(i)
	}
	return time.Duration(retMS) * time.Millisecond
}
