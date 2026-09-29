package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Serving is configuration for how the interceptor serves the proxy
// and admin server
type Serving struct {
	// WatchNamespace is the namespace to watch for new HTTPScaledObjects.
	// Leave this empty to watch HTTPScaledObjects in all namespaces.
	WatchNamespace string `env:"KEDA_HTTP_WATCH_NAMESPACE" envDefault:""`
	// ProxyPort is the port that the public proxy should run on
	ProxyPort int `env:"KEDA_HTTP_PROXY_PORT,required"`
	// AdminPort is the port that the internal admin server should run on.
	// This is the server that the external scaler will issue metrics
	// requests to
	AdminPort int `env:"KEDA_HTTP_ADMIN_PORT,required"`
	// CacheSyncPeriod is the time interval for the controller-runtime cache to resync.
	// TODO: consider removing this to use the default value, otherwise align the env var name
	CacheSyncPeriod time.Duration `env:"KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD" envDefault:"60m"`

	// ProxyTLSEnabled is a flag to specify whether the interceptor proxy should
	// be running using a TLS enabled server
	ProxyTLSEnabled bool `env:"KEDA_HTTP_PROXY_TLS_ENABLED" envDefault:"false"`
	// TLSCertPath is the path to read the certificate file from for the TLS server
	TLSCertPath string `env:"KEDA_HTTP_PROXY_TLS_CERT_PATH" envDefault:"/certs/tls.crt"`
	// TLSKeyPath is the path to read the private key file from for the TLS server
	TLSKeyPath string `env:"KEDA_HTTP_PROXY_TLS_KEY_PATH" envDefault:"/certs/tls.key"`
	// TLSCertStorePaths is a comma separated list of paths to read the certificate/key pairs for the TLS server
	TLSCertStorePaths string `env:"KEDA_HTTP_PROXY_TLS_CERT_STORE_PATHS" envDefault:""`
	// TLSPort is the port that the server should serve on if TLS is enabled
	TLSPort int `env:"KEDA_HTTP_PROXY_TLS_PORT" envDefault:"8443"`

	// ProfilingAddr if not empty, pprof will be available on this address, assuming host:port here
	ProfilingAddr string `env:"PROFILING_BIND_ADDRESS" envDefault:""`
	// EnableColdStartHeader enables/disables the X-KEDA-HTTP-Cold-Start response header
	EnableColdStartHeader bool `env:"KEDA_HTTP_ENABLE_COLD_START_HEADER" envDefault:"true"`
	// LogRequests enables/disables logging of incoming requests
	LogRequests bool `env:"KEDA_HTTP_LOG_REQUESTS" envDefault:"false"`

	// DrainTimeout is the maximum time to wait for in-flight requests to
	// complete after the proxy listener is closed. If 0, waits indefinitely
	// (bounded only by terminationGracePeriodSeconds).
	DrainTimeout time.Duration `env:"KEDA_HTTP_DRAIN_TIMEOUT" envDefault:"30s"`
	// ShutdownDelay is the time between receiving SIGTERM and closing the proxy
	// listener. During this window the readiness probe returns 503 while the
	// server continues serving in-flight and new requests, giving Kubernetes
	// time to propagate endpoint removal.
	ShutdownDelay time.Duration `env:"KEDA_HTTP_SHUTDOWN_DELAY" envDefault:"5s"`
	// DirectPodRouting routes requests to a ready pod IP instead of the Service
	// ClusterIP, bypassing kube-proxy and other Service-layer features
	// (NetworkPolicy, session affinity, topology-aware routing).
	DirectPodRouting bool `env:"KEDA_HTTP_DIRECT_POD_ROUTING" envDefault:"true"`

	// ColdStartMaxPendingRequests is the default limit on requests held per
	// route while its backend has no ready endpoints (e.g. during
	// scale-from-zero). Additional requests are rejected with 503 or served
	// the route's placeholder response, per the route's coldStart.overflow.
	// Routes override this default via coldStart.maxPendingRequests.
	// 0 means unlimited. The limit applies per interceptor replica: the
	// effective cluster-wide capacity is this value multiplied by the
	// number of interceptor replicas.
	ColdStartMaxPendingRequests int `env:"KEDA_HTTP_COLD_START_MAX_PENDING_REQUESTS" envDefault:"0"`

	// RoutingTableRefreshInterval is how often the routing table rebuilds
	// itself on top of the rebuilds triggered by InterceptorRoute and
	// HTTPScaledObject events. The periodic rebuild is what lets readiness
	// distinguish a stalled refresh loop from an interceptor that simply has
	// no route changes to react to. 0 disables periodic rebuilds.
	RoutingTableRefreshInterval time.Duration `env:"KEDA_HTTP_ROUTING_TABLE_REFRESH_INTERVAL" envDefault:"1m"`
	// RoutingTableMaxStaleness is how long the routing table may go without a
	// successful rebuild before /readyz starts failing, taking the pod out of
	// the Service. It must be at least twice RoutingTableRefreshInterval so a
	// single missed rebuild is tolerated. 0 disables the check, restoring the
	// previous behavior where readiness only reflects the initial sync.
	//
	// Raising this trades slower detection for a smaller chance of pulling
	// healthy pods out of rotation; the scaler polls only ready interceptors,
	// so a false positive across every replica reports zero pending requests
	// and scales the workloads down.
	RoutingTableMaxStaleness time.Duration `env:"KEDA_HTTP_ROUTING_TABLE_MAX_STALENESS" envDefault:"5m"`
}

// MustParseServing parses standard configs and returns the
// newly created config. It panics if parsing fails.
func MustParseServing() Serving {
	cfg := env.Must(env.ParseAs[Serving]())

	if err := cfg.validate(); err != nil {
		panic(fmt.Errorf("invalid serving config: %w", err))
	}

	return cfg
}

func (s Serving) validate() error {
	if s.RoutingTableRefreshInterval < 0 {
		return fmt.Errorf("KEDA_HTTP_ROUTING_TABLE_REFRESH_INTERVAL must not be negative, got %s", s.RoutingTableRefreshInterval)
	}
	if s.RoutingTableMaxStaleness < 0 {
		return fmt.Errorf("KEDA_HTTP_ROUTING_TABLE_MAX_STALENESS must not be negative, got %s", s.RoutingTableMaxStaleness)
	}

	// Guard the two combinations that would fail readiness on a healthy pod:
	// no periodic rebuild to advance the timestamp, and a deadline too tight
	// to survive one missed rebuild.
	if s.RoutingTableMaxStaleness > 0 {
		if s.RoutingTableRefreshInterval == 0 {
			return fmt.Errorf(
				"KEDA_HTTP_ROUTING_TABLE_MAX_STALENESS (%s) needs KEDA_HTTP_ROUTING_TABLE_REFRESH_INTERVAL to be set, otherwise an interceptor with no route changes fails readiness",
				s.RoutingTableMaxStaleness,
			)
		}
		if s.RoutingTableMaxStaleness < 2*s.RoutingTableRefreshInterval {
			return fmt.Errorf(
				"KEDA_HTTP_ROUTING_TABLE_MAX_STALENESS (%s) must be at least twice KEDA_HTTP_ROUTING_TABLE_REFRESH_INTERVAL (%s), otherwise a single missed rebuild fails readiness",
				s.RoutingTableMaxStaleness, s.RoutingTableRefreshInterval,
			)
		}
	}

	return nil
}
