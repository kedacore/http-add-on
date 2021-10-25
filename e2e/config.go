package e2e

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

type config struct {
	// Required configurations for all tests
	Namespace  string `envconfig:"NAMESPACE" required:"true"`
	IngAddress string `envconfig:"INGRESS_ADDRESS" required:"true"`
	// Optional configurations
	NumReqsAgainstProxy int           `envconfig:"NUM_REQUESTS_TO_EXECUTE" default:"10000"`
	ProxyAdminSvc       string        `envconfig:"PROXY_ADMIN_SERVICE_NAME" default:"keda-add-ons-http-interceptor-proxy"`
	ProxyAdminPort      int           `envconfig:"PROXY_ADMIN_PORT" default:"8080"`
	ScalerAdminSvc      string        `envconfig:"SCALER_ADMIN_SERVICE_NAME" default:"keda-add-ons-http-external-scaler"`
	ScalerAdminPort     int           `envconfig:"SCALER_ADMIN_PORT" default:"9091"`
	OperatorAdminSvc    string        `envconfig:"OPERATOR_ADMIN_SERVICE_NAME" default:"keda-add-ons-http-operator-admin"`
	OperatorAdminPort   int           `envconfig:"OPERATOR_ADMIN_PORT" default:"9090"`
	AdminServerCheckDur time.Duration `envconfig:"ADMIN_SERVER_CHECK_DUR" default:"500ms"`
}

func (c config) namespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	return fmt.Sprintf(
		"keda-http-add-on-e2e-%s",
		uuid.NewString(),
	)
}

func parseConfig() (config, bool, error) {
	const runE2EEnvVar = "KEDA_HTTP_E2E_SHOULD_RUN"
	shouldRun := os.Getenv(runE2EEnvVar)
	if shouldRun != "true" {
		return config{}, false, nil
	}
	cfg := config{}
	processErr := envconfig.Process("KEDA_HTTP_E2E", &cfg)
	return cfg, true, processErr
}
