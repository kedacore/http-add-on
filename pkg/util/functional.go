package util

import (
	"context"
	"fmt"
	"time"
)

//revive:disable:context-as-argument
func ApplyContext(f func(ctx context.Context) error, ctx context.Context) func() error {
	//revive:enable:context-as-argument
	return func() error {
		return f(ctx)
	}
}

func DeapplyError(f func(), err error) func() error {
	return func() error {
		f()
		return err
	}
}

func IgnoringError(f func() error) {
	_ = f()
}

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
