package main

import "github.com/kelseyhightower/envconfig"

type config struct {
	// GRPCPort is what port to serve the KEDA-compatible gRPC external scaler interface
	// on
	GRPCPort int `envconfig:"KEDA_HTTP_SCALER_PORT" default:"8080"`
	// HealthPort is the port to serve the Kubernetes health check endpoints on
	HealthPort int `envconfig:"KEDA_HTTP_HEALTH_PORT" default:"8090"`
	// TargetNamespace is the namespace in which this scaler is running, and the namespace
	// that the target interceptors are running in. This scaler and all the interceptors
	// must be running in the same namespace
	TargetNamespace string `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE" required:"true"`
	// TargetService is the name of the service to issue metrics RPC requests to interceptors
	TargetService string `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE" required:"true"`
	// TargetPort is the port on TargetService to which to issue metrics RPC requests to
	// interceptors
	TargetPort int `envconfig:"KEDA_HTTP_SCALER_TARGET_ADMIN_PORT" required:"true"`
}

func mustParseConfig() *config {
	ret := new(config)
	envconfig.MustProcess("", ret)
	return ret
}
