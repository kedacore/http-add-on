package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	ServiceName string `env:"KEDA_HTTP_OPERATOR_EXTERNAL_SCALER_SERVICE,required"`
	Port        int32  `env:"KEDA_HTTP_OPERATOR_EXTERNAL_SCALER_PORT" envDefault:"8091"`
}

type Base struct {
	// The current namespace in which the operator is running.
	CurrentNamespace string `env:"KEDA_HTTP_OPERATOR_NAMESPACE" envDefault:""`
	// The namespace the operator should watch. Leave blank to
	// tell the operator to watch all namespaces.
	WatchNamespace string `env:"KEDA_HTTP_OPERATOR_WATCH_NAMESPACE" envDefault:""`
	// Leader election durations. Nil means use controller-runtime defaults.
	LeaseDuration *time.Duration `env:"KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION"`
	RenewDeadline *time.Duration `env:"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE"`
	RetryPeriod   *time.Duration `env:"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD"`
}

func NewBaseFromEnv() (Base, error) {
	return env.ParseAs[Base]()
}

func (e ExternalScaler) HostName(namespace string) string {
	return fmt.Sprintf(
		"%s.%s:%d",
		e.ServiceName,
		namespace,
		e.Port,
	)
}

// Deprecated env var names (missing underscore after KEDA).
// TODO: remove in v0.16.0
var externalScalerDeprecatedEnvVars = map[string]string{
	"KEDAHTTP_OPERATOR_EXTERNAL_SCALER_SERVICE": "KEDA_HTTP_OPERATOR_EXTERNAL_SCALER_SERVICE",
	"KEDAHTTP_OPERATOR_EXTERNAL_SCALER_PORT":    "KEDA_HTTP_OPERATOR_EXTERNAL_SCALER_PORT",
}

func NewExternalScalerFromEnv() (ExternalScaler, error) {
	logger := log.Log.WithName("setup")
	for oldKey, newKey := range externalScalerDeprecatedEnvVars {
		if v, ok := os.LookupEnv(oldKey); ok {
			logger.Info("environment variable is deprecated, use the new name instead",
				"old", oldKey, "new", newKey)
			if _, set := os.LookupEnv(newKey); !set {
				if err := os.Setenv(newKey, v); err != nil {
					logger.Error(err, "failed to set environment variable", "key", newKey)
				}
			}
		}
	}
	return env.ParseAs[ExternalScaler]()
}
