package config

import (
	"fmt"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

// Interceptor holds static configuration info for the
// interceptor
type Interceptor struct {
	ServiceName string `envconfig:"SERVICE" required:"true"`
	ProxyPort   int32  `envconfig:"PROXY_PORT" default:"8091"`
	AdminPort   int32  `envconfig:"ADMIN_PORT" default:"8090"`
}

// ExternalScaler holds static configuration info for the
// external scaler
type ExternalScaler struct {
	ServiceName string `envconfig:"SERVICE_NAME" required:"true"`
	Port        int32  `envconfig:"PORT" required:"true"`
}

// Base contains foundational configuration required for the
// operator to function.
type Base struct {
	TargetPendingRequests int32 `envconfig:"TARGET_PENDING_REQUESTS" default:"100"`
	// The current namespace in which the operator is running.
	CurrentNamespace string `envconfig:"NAMESPACE" default:""`
	// The namespace the operator should watch. Leave blank to
	// tell the operator to watch all namespaces.
	WatchNamespace string `envconfig:"WATCH_NAMESPACE" default:""`
}

// NewBaseFromEnv parses appropriate environment variables
// and returns a Base struct to match those values.
//
// Returns nil and an appropriate error if any required
// values were missing or malformed.
func NewBaseFromEnv() (*Base, error) {
	ret := new(Base)
	if err := envconfig.Process(
		"KEDA_HTTP_OPERATOR",
		ret,
	); err != nil {
		return nil, err
	}
	return ret, nil
}

// HostName returns the Kubernetes in-cluster hostname
// qualified with the given namespace.
// For example, if e.ServiceName is "mysvc", e.Port
// is 8080, and you pass "myns" as the namespace parameter,
// this function will return "mysvc.myns:8080"
func (e ExternalScaler) HostName(namespace string) string {
	return fmt.Sprintf(
		"%s.%s:%d",
		e.ServiceName,
		namespace,
		e.Port,
	)
}

// AdminPortString returns i.AdminPort in string format, rather than
// as an int32.
func (i Interceptor) AdminPortString() string {
	return strconv.Itoa(int(i.AdminPort))
}

// NewInterceptorFromEnv gets interceptor configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewInterceptorFromEnv() (*Interceptor, error) {
	ret := &Interceptor{}
	if err := envconfig.Process("KEDAHTTP_INTERCEPTOR", ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// NewExternalScalerFromEnv gets external scaler configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewExternalScalerFromEnv() (*ExternalScaler, error) {
	ret := &ExternalScaler{}
	if err := envconfig.Process(
		"KEDAHTTP_OPERATOR_EXTERNAL_SCALER",
		ret,
	); err != nil {
		return nil, err
	}
	return ret, nil
}
