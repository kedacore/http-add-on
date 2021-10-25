package e2e

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type harness struct {
	context.Context
	done    func()
	cfg     config
	cl      client.Client
	restCfg *rest.Config
}

func (h *harness) close() {
	h.done()
}

func setup() (*harness, bool, error) {
	cfg, shouldRun, parseCfgErr := parseConfig()
	if !shouldRun {
		return nil, false, nil
	}

	cl, restCfg, err := getClient()
	if err != nil {
		return nil, true, err
	}
	ctx, cancel := context.WithCancel(context.Background())

	return &harness{
		Context: ctx,
		done:    cancel,
		cfg:     cfg,
		cl:      cl,
		restCfg: restCfg,
	}, shouldRun, parseCfgErr
}
