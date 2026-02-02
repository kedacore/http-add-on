package config

import (
	"fmt"
	"time"
)

func Validate(srvCfg Serving, timeoutsCfg Timeouts) error {
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
