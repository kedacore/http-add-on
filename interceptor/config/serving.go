package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Serving is configuration for how the interceptor serves the proxy
// and admin server
type Serving struct {
	// CurrentNamespace is the namespace that the interceptor is
	// currently running in
	CurrentNamespace string `envconfig:"KEDA_HTTP_CURRENT_NAMESPACE" required:"true"`
	// ProxyPort is the port that the public proxy should run on
	ProxyPort int `envconfig:"KEDA_HTTP_PROXY_PORT" required:"true"`
	// AdminPort is the port that the internal admin server should run on.
	// This is the server that the external scaler will issue metrics
	// requests to
	AdminPort int `envconfig:"KEDA_HTTP_ADMIN_PORT" required:"true"`
	// RoutingTableUpdateDurationMS is the interval (in milliseconds) representing how
	// often to do a complete update of the routing table ConfigMap.
	//
	// The interceptor will also open a watch stream to the routing table
	// ConfigMap and attempt to update the routing table on every update.
	//
	// Since it does full updates alongside watch stream updates, it can
	// only process one at a time. Therefore, this is a best effort timeout
	RoutingTableUpdateDurationMS int `envconfig:"KEDA_HTTP_ROUTING_TABLE_UPDATE_DURATION_MS" default:"500"`
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
