package e2e

import "time"

// tickMax tries to run f up to maxTries times. it fails
// with the most recent error that f() returned
// after either tickDur passes or it has tried
// maxTries calls
func tickMax(
	maxTries int,
	tickDur time.Duration,
	f func() error,
) error {
	var lastErr error
	ticker := time.NewTicker(tickDur)
	defer ticker.Stop()
	for i := 0; i < maxTries; i++ {
		<-ticker.C
		err := f()
		if err == nil {
			lastErr = nil
			break
		} else {
			lastErr = err
		}
	}
	return lastErr
}
