package main

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type config struct {
	// GRPCPort is what port to serve the KEDA-compatible gRPC external scaler interface
	// on
	GRPCPort int `env:"KEDA_HTTP_SCALER_PORT" envDefault:"8080"`
	// TargetNamespace is the namespace in which this scaler is running, and the namespace
	// that the target interceptors are running in. This scaler and all the interceptors
	// must be running in the same namespace
	TargetNamespace string `env:"KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE,required"`
	// TargetService is the name of the service to issue metrics RPC requests to interceptors
	TargetService string `env:"KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE,required"`
	// TargetDeployment is the name of the deployment to issue metrics RPC requests to interceptors
	TargetDeployment string `env:"KEDA_HTTP_SCALER_TARGET_ADMIN_DEPLOYMENT,required"`
	// TargetPort is the port on TargetService to which to issue metrics RPC requests to
	// interceptors
	TargetPort int `env:"KEDA_HTTP_SCALER_TARGET_ADMIN_PORT,required"`
	// CacheSyncPeriod is the time interval for the controller-runtime cache to resync.
	// TODO: consider removing this to use the default value, otherwise align the env var name
	CacheSyncPeriod time.Duration `env:"KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD" envDefault:"60m"`
	// QueueTickDuration is the duration between queue requests
	QueueTickDuration time.Duration `env:"KEDA_HTTP_QUEUE_TICK_DURATION" envDefault:"500ms"`
	// ProfilingAddr if not empty, pprof will be available on this address, assuming host:port here
	ProfilingAddr string `env:"PROFILING_BIND_ADDRESS" envDefault:""`
	// StreamIntervalMS is the interval in milliseconds between stream ticks
	StreamIntervalMS int `env:"KEDA_HTTP_SCALER_STREAM_INTERVAL_MS" envDefault:"200"`
}

func mustParseConfig() config {
	return env.Must(env.ParseAs[config]())
}
