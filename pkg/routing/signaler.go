package routing

import (
	"context"
)

type Signaler interface {
	Signal()
	Wait(ctx context.Context) error
}

type signaler chan struct{}

func NewSignaler() Signaler {
	return make(signaler, 1)
}

var _ Signaler = (*signaler)(nil)

func (s signaler) Signal() {
	select {
	case s <- struct{}{}:
	default:
	}
}

func (s signaler) Wait(ctx context.Context) error {
	select {
	case <-s:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
