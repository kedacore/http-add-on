package util

import (
	"context"
)

type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

type HealthCheckerFunc func(ctx context.Context) error

var _ HealthChecker = (*HealthCheckerFunc)(nil)

func (f HealthCheckerFunc) HealthCheck(ctx context.Context) error {
	return f(ctx)
}
