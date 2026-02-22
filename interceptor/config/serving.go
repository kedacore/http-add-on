package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Serving is configuration for how the interceptor serves the proxy
// and admin server
type Serving struct {
	// WatchNamespace is the namespace to watch for new HTTPScaledObjects.
	// Leave this empty to watch HTTPScaledObjects in all namespaces.
	WatchNamespace string `envconfig:"KEDA_HTTP_WATCH_NAMESPACE" default:""`
	// ProxyPort is the port that the public proxy should run on
	ProxyPort int `envconfig:"KEDA_HTTP_PROXY_PORT" required:"true"`
	// AdminPort is the port that the internal admin server should run on.
	// This is the server that the external scaler will issue metrics
	// requests to
	AdminPort int `envconfig:"KEDA_HTTP_ADMIN_PORT" required:"true"`
	// CacheSyncPeriod is the time interval for the controller-runtime cache to resync.
	// TODO: consider removing this to use the default value, otherwise align the env var name
	CacheSyncPeriod time.Duration `envconfig:"KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD" default:"60m"`
	// The interceptor has an internal process that periodically fetches the state
	// of endpoints that is running the servers it forwards to.
	//
	// This is the interval (in milliseconds) representing how often to do a fetch
	// TODO: this is actually the informer resync period, not a poll interval, default is too aggressive
	EndpointsCachePollIntervalMS int `envconfig:"KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS" default:"1000"`
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
	// ProfilingAddr if not empty, pprof will be available on this address, assuming host:port here
	ProfilingAddr string `envconfig:"PROFILING_BIND_ADDRESS" default:""`
	// EnableColdStartHeader enables/disables the X-KEDA-HTTP-Cold-Start response header
	EnableColdStartHeader bool `envconfig:"KEDA_HTTP_ENABLE_COLD_START_HEADER" default:"true"`
	// LogRequests enables/disables logging of incoming requests
	LogRequests bool `envconfig:"KEDA_HTTP_LOG_REQUESTS" default:"false"`
}

// MustParseServing parses standard configs using envconfig and returns the
// newly created config. It panics if parsing fails.
func MustParseServing() Serving {
	var ret Serving
	envconfig.MustProcess("", &ret)
	return ret
}
