package handler

import (
	"context"
)

type ProbeHealthCheck func(ctx context.Context) error
