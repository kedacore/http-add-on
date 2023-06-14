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
	// The interceptor has an internal process that periodically fetches the state
	// of deployment that is running the servers it forwards to.
	//
	// This is the interval (in milliseconds) representing how often to do a fetch
	DeploymentCachePollIntervalMS int `envconfig:"KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS" default:"250"`
}

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseServing() *Serving {
	ret := new(Serving)
	envconfig.MustProcess("", ret)
	return ret
}
