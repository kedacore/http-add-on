package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
)

func Validate(srvCfg *Serving, timeoutsCfg Timeouts, lggr logr.Logger) error {
	// TODO(jorturfer): delete this for v0.9.0
	_, deploymentEnvExist := os.LookupEnv("KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS")
	_, endpointsEnvExist := os.LookupEnv("KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS")
	if deploymentEnvExist && endpointsEnvExist {
		return fmt.Errorf(
			"%s and %s are mutual exclusive",
			"KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS",
			"KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS",
		)
	}
	if deploymentEnvExist && !endpointsEnvExist {
		srvCfg.EndpointsCachePollIntervalMS = srvCfg.DeploymentCachePollIntervalMS
		srvCfg.DeploymentCachePollIntervalMS = 0
		lggr.Info("WARNING: KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS has been deprecated in favor of KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS and wil be removed for v0.9.0")
	}
	// END TODO

	endpointsCachePollInterval := time.Duration(srvCfg.EndpointsCachePollIntervalMS) * time.Millisecond
	if timeoutsCfg.WorkloadReplicas < endpointsCachePollInterval {
		return fmt.Errorf(
			"workload replicas timeout (%s) should not be less than the Endpoints Cache Poll Interval (%s)",
			timeoutsCfg.WorkloadReplicas,
			endpointsCachePollInterval,
		)
	}
	return nil
}
