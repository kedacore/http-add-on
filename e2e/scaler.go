package e2e

import (
	"context"
	"errors"

	"k8s.io/client-go/rest"
)

func checkScalerMetrics(
	ctx context.Context,
	restCfg *rest.Config,
	ns,
	svcName string,
	port int,
) error {
	return errors.New("TODO")
}