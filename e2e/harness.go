package e2e

import (
	"context"
)

type harness struct {
	context.Context
	done func()
	cfg  config
}

func (h *harness) close() {
	h.done()
}

func setup() (*harness, bool, error) {
	cfg, shouldRun, parseCfgErr := parseConfig()

	ctx, cancel := context.WithCancel(context.Background())
	return &harness{
		Context: ctx,
		done:    cancel,
		cfg:     cfg,
	}, shouldRun, parseCfgErr
}
