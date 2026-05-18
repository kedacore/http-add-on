package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// DirectPodRoutingMode controls whether and when the interceptor routes requests
// directly to a ready pod IP instead of through the ClusterIP service.
type DirectPodRoutingMode string

const (
	// DirectPodRoutingDisabled never bypasses the ClusterIP service (default).
	DirectPodRoutingDisabled DirectPodRoutingMode = "disabled"
	// DirectPodRoutingColdStartOnly bypasses the ClusterIP service only on cold
	// starts, reducing latency when kube-proxy rules are slow to propagate.
	DirectPodRoutingColdStartOnly DirectPodRoutingMode = "cold-start-only"
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
	// TLSSkipVerify is a boolean flag to specify whether the interceptor should skip TLS verification for upstreams
	TLSSkipVerify bool `env:"KEDA_HTTP_PROXY_TLS_SKIP_VERIFY" envDefault:"false"`
	// TLSPort is the port that the server should serve on if TLS is enabled
	TLSPort int `env:"KEDA_HTTP_PROXY_TLS_PORT" envDefault:"8443"`
	// TLSMinVersion is the minimum TLS version to accept ("1.2" or "1.3").
	// If empty, the Go default is used (currently TLS 1.2).
	TLSMinVersion string `env:"KEDA_HTTP_PROXY_TLS_MIN_VERSION" envDefault:""`
	// TLSMaxVersion is the maximum TLS version to accept ("1.2" or "1.3").
	// Defaults to the highest version supported by crypto/tls if empty.
	TLSMaxVersion string `env:"KEDA_HTTP_PROXY_TLS_MAX_VERSION" envDefault:""`
	// TLSCipherSuites is a comma-separated list of TLS cipher suite names
	// (e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384").
	// If empty, the default Go cipher suites are used.
	TLSCipherSuites string `env:"KEDA_HTTP_PROXY_TLS_CIPHER_SUITES" envDefault:""`
	// TLSCurvePreferences is a comma-separated list of elliptic curve names
	// (e.g. "X25519,CurveP256"). If empty, the default Go curve preferences are used.
	TLSCurvePreferences string `env:"KEDA_HTTP_PROXY_TLS_CURVE_PREFERENCES" envDefault:""`
	// ProfilingAddr if not empty, pprof will be available on this address, assuming host:port here
	ProfilingAddr string `env:"PROFILING_BIND_ADDRESS" envDefault:""`
	// EnableColdStartHeader enables/disables the X-KEDA-HTTP-Cold-Start response header
	EnableColdStartHeader bool `env:"KEDA_HTTP_ENABLE_COLD_START_HEADER" envDefault:"true"`
	// LogRequests enables/disables logging of incoming requests
	LogRequests bool `env:"KEDA_HTTP_LOG_REQUESTS" envDefault:"false"`
	// DirectPodRouting controls when the interceptor routes directly to a pod IP
	// instead of the ClusterIP service. Valid values: "disabled", "cold-start-only".
	DirectPodRouting DirectPodRoutingMode `env:"KEDA_HTTP_DIRECT_POD_ROUTING" envDefault:"disabled"`
}

// MustParseServing parses standard configs and returns the
// newly created config. It panics if parsing fails.
func MustParseServing() Serving {
	s := env.Must(env.ParseAs[Serving]())
	switch s.DirectPodRouting {
	case DirectPodRoutingDisabled, DirectPodRoutingColdStartOnly:
		// valid
	default:
		panic(fmt.Sprintf("invalid KEDA_HTTP_DIRECT_POD_ROUTING value %q: must be %q or %q",
			s.DirectPodRouting, DirectPodRoutingDisabled, DirectPodRoutingColdStartOnly))
	}
	return s
}
