package e2e

import (
	"testing"
	"time"
)

func remainingDurInTest(
	t *testing.T,
	def time.Duration,
) time.Duration {
	expireTime, ok := t.Deadline()
	if !ok {
		return def
	}
	if time.Now().After(expireTime) {
		return 0
	}
	return time.Until(expireTime)
}
