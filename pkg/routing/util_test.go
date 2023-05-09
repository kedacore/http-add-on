package routing

import (
	"fmt"
	"time"
)

func deapplyError(f func(), err error) func() error {
	return func() error {
		f()
		return err
	}
}

func ignoringError(f func() error) {
	_ = f()
}

func withTimeout(d time.Duration, f func() error) error {
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
