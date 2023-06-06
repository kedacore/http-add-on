package util

import (
	"fmt"
	"time"
)

func WithTimeout(d time.Duration, f func() error) error {
	errs := make(chan error)
	defer close(errs)

	go func() {
		errs <- f()
	}()

	select {
	case err := <-errs:
		return err
	case <-time.After(d):
		return fmt.Errorf("timed out after %v", d)
	}
}
