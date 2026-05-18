package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

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
