package config

import (
	"fmt"
	"time"
)

func Validate(srvCfg Serving, timeoutsCfg Timeouts) error {
	deplCachePollInterval := time.Duration(srvCfg.DeploymentCachePollIntervalMS) * time.Millisecond
	if timeoutsCfg.DeploymentReplicas < deplCachePollInterval {
		return fmt.Errorf(
			"deployment replicas timeout (%s) should not be less than the Deployment Cache Poll Interval (%s)",
			timeoutsCfg.DeploymentReplicas,
			deplCachePollInterval,
		)

	}
	return nil
}
