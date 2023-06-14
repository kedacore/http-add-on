package util

import (
	"context"
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
