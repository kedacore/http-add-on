package e2e

import (
	"context"
	"fmt"
	"time"
)

func waitUntil(
	ctx context.Context,
	dur time.Duration,
	fn func(context.Context) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()
	lastErr := fmt.Errorf(
		"timeout after %s waiting for condition",
		dur,
	)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"timed out after %s. last error: %w",
				dur,
				lastErr,
			)
		default:
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
	}
}
