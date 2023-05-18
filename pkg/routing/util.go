package routing

import (
	"context"
)

//revive:disable:context-as-argument
func applyContext(f func(ctx context.Context) error, ctx context.Context) func() error {
	//revive:enable:context-as-argument
	return func() error {
		return f(ctx)
	}
}
