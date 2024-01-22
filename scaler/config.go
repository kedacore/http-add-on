package main

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	// GRPCPort is what port to serve the KEDA-compatible gRPC external scaler interface
	// on
	GRPCPort int `envconfig:"KEDA_HTTP_SCALER_PORT" default:"8080"`
	// TargetNamespace is the namespace in which this scaler is running, and the namespace
	// that the target interceptors are running in. This scaler and all the interceptors
	// must be running in the same namespace
	TargetNamespace string `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE" required:"true"`
	// TargetService is the name of the service to issue metrics RPC requests to interceptors
	TargetService string `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE" required:"true"`
	// TargetDeployment is the name of the deployment to issue metrics RPC requests to interceptors
	TargetDeployment string `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_DEPLOYMENT" required:"true"`
	// TargetPort is the port on TargetService to which to issue metrics RPC requests to
	// interceptors
	TargetPort int `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_PORT" required:"true"`
	// TargetPendingRequests is the default value for the
	// pending requests value that the scaler will return to
	// KEDA, if that value is not set on an incoming
	// `HTTPScaledObject`
	TargetPendingRequests int `envconfig:"KEDA_HTTP_SCALER_TARGET_PENDING_REQUESTS" default:"100"`
	// ConfigMapCacheRsyncPeriod is the time interval
	// for the configmap informer to rsync the local cache.
	ConfigMapCacheRsyncPeriod time.Duration `envconfig:"KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD" default:"60m"`
	// DeploymentCacheRsyncPeriod is the time interval
	// for the deployment informer to rsync the local cache.
	DeploymentCacheRsyncPeriod time.Duration `envconfig:"KEDA_HTTP_SCALER_DEPLOYMENT_INFORMER_RSYNC_PERIOD" default:"60m"`
	// QueueTickDuration is the duration between queue requests
	QueueTickDuration time.Duration `envconfig:"KEDA_HTTP_QUEUE_TICK_DURATION" default:"500ms"`
}

func mustParseConfig() *config {
	ret := new(config)
	envconfig.MustProcess("", ret)
	return ret
}
