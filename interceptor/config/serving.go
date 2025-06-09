package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Serving is configuration for how the interceptor serves the proxy
// and admin server
type Serving struct {
	// CurrentNamespace is the namespace that the interceptor is
	// currently running in
	CurrentNamespace string `envconfig:"KEDA_HTTP_CURRENT_NAMESPACE" required:"true"`
	// WatchNamespace is the namespace to watch for new HTTPScaledObjects.
	// Leave this empty to watch HTTPScaledObjects in all namespaces.
	WatchNamespace string `envconfig:"KEDA_HTTP_WATCH_NAMESPACE" default:""`
	// ProxyPort is the port that the public proxy should run on
	ProxyPort int `envconfig:"KEDA_HTTP_PROXY_PORT" required:"true"`
	// AdminPort is the port that the internal admin server should run on.
	// This is the server that the external scaler will issue metrics
	// requests to
	AdminPort int `envconfig:"KEDA_HTTP_ADMIN_PORT" required:"true"`
	// ConfigMapCacheRsyncPeriod is the time interval
	// for the configmap informer to rsync the local cache.
	ConfigMapCacheRsyncPeriod time.Duration `envconfig:"KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD" default:"60m"`
	// Deprecated: The interceptor has an internal process that periodically fetches the state
	// of deployment that is running the servers it forwards to.
	//
	// This is the interval (in milliseconds) representing how often to do a fetch
	DeploymentCachePollIntervalMS int `envconfig:"KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS" default:"250"`
	// The interceptor has an internal process that periodically fetches the state
	// of endpoints that is running the servers it forwards to.
	//
	// This is the interval (in milliseconds) representing how often to do a fetch
	EndpointsCachePollIntervalMS int `envconfig:"KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS" default:"250"`
	// ProxyTLSEnabled is a flag to specify whether the interceptor proxy should
	// be running using a TLS enabled server
	ProxyTLSEnabled bool `envconfig:"KEDA_HTTP_PROXY_TLS_ENABLED" default:"false"`
	// TLSCertPath is the path to read the certificate file from for the TLS server
	TLSCertPath string `envconfig:"KEDA_HTTP_PROXY_TLS_CERT_PATH" default:"/certs/tls.crt"`
	// TLSKeyPath is the path to read the private key file from for the TLS server
	TLSKeyPath string `envconfig:"KEDA_HTTP_PROXY_TLS_KEY_PATH" default:"/certs/tls.key"`
	// TLSCertStorePaths is a comma separated list of paths to read the certificate/key pairs for the TLS server
	TLSCertStorePaths string `envconfig:"KEDA_HTTP_PROXY_TLS_CERT_STORE_PATHS" default:""`
	// TLSSkipVerify is a boolean flag to specify whether the interceptor should skip TLS verification for upstreams
	TLSSkipVerify bool `envconfig:"KEDA_HTTP_PROXY_TLS_SKIP_VERIFY" default:"false"`
	// TLSPort is the port that the server should serve on if TLS is enabled
	TLSPort int `envconfig:"KEDA_HTTP_PROXY_TLS_PORT" default:"8443"`
}

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseServing() *Serving {
	ret := new(Serving)
	envconfig.MustProcess("", ret)
	return ret
}
