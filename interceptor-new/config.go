package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds all interceptor configuration, parsed from environment variables.
// Env var names match the existing interceptor for drop-in compatibility.
type Config struct {
	ProxyPort             int
	AdminPort             int
	MetricsPort           int
	ConnectTimeout        time.Duration
	KeepAlive             time.Duration
	ResponseHeaderTimeout time.Duration
	ConditionWaitTimeout  time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DNSCacheTTL           time.Duration
	ForceHTTP2            bool
	WatchNamespace        string
	LogRequests           bool
	EnableColdStartHeader bool

	// TLS
	TLSEnabled        bool
	TLSProxyPort      int
	TLSCertPath       string
	TLSKeyPath        string
	TLSCertStorePaths string
	TLSSkipVerify     bool

	// Observability
	TracingEnabled      bool
	TracingExporter     string // "http/protobuf", "grpc", or "console"
	PromExporterEnabled bool
	OtelMetricsEnabled  bool
	ProfilingAddr       string // e.g. ":6060"; empty = disabled
}

const (
	defaultNamespace = "default"
)

// ConfigFromEnv reads all configuration from environment variables with sensible defaults.
func ConfigFromEnv() Config {
	return Config{
		ProxyPort:             envInt("KEDA_HTTP_PROXY_PORT", 8080),
		AdminPort:             envInt("KEDA_HTTP_ADMIN_PORT", 9090),
		MetricsPort:           envInt("OTEL_PROM_EXPORTER_PORT", 2223),
		ConnectTimeout:        envDuration("KEDA_HTTP_CONNECT_TIMEOUT", 500*time.Millisecond),
		KeepAlive:             envDuration("KEDA_HTTP_KEEP_ALIVE", 1*time.Second),
		ResponseHeaderTimeout: envDuration("KEDA_RESPONSE_HEADER_TIMEOUT", 500*time.Millisecond),
		ConditionWaitTimeout:  envDuration("KEDA_CONDITION_WAIT_TIMEOUT", 20*time.Second),
		MaxIdleConns:          envInt("KEDA_HTTP_MAX_IDLE_CONNS", 100),
		MaxIdleConnsPerHost:   envInt("KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST", 20),
		IdleConnTimeout:       envDuration("KEDA_HTTP_IDLE_CONN_TIMEOUT", 90*time.Second),
		TLSHandshakeTimeout:   envDuration("KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT", 10*time.Second),
		ExpectContinueTimeout: envDuration("KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT", 1*time.Second),
		DNSCacheTTL:           envDuration("KEDA_HTTP_DNS_CACHE_TTL", 30*time.Second),
		ForceHTTP2:            envBool("KEDA_HTTP_FORCE_HTTP2"),
		WatchNamespace:        os.Getenv("KEDA_HTTP_WATCH_NAMESPACE"),
		LogRequests:           envBool("KEDA_HTTP_LOG_REQUESTS"),
		EnableColdStartHeader: envBoolDefault("KEDA_HTTP_ENABLE_COLD_START_HEADER", true),

		// TLS
		TLSEnabled:        envBool("KEDA_HTTP_PROXY_TLS_ENABLED"),
		TLSProxyPort:      envInt("KEDA_HTTP_PROXY_TLS_PORT", 8443),
		TLSCertPath:       envString("KEDA_HTTP_PROXY_TLS_CERT_PATH", "/certs/tls.crt"),
		TLSKeyPath:        envString("KEDA_HTTP_PROXY_TLS_KEY_PATH", "/certs/tls.key"),
		TLSCertStorePaths: os.Getenv("KEDA_HTTP_PROXY_TLS_CERT_STORE_PATHS"),
		TLSSkipVerify:     envBool("KEDA_HTTP_PROXY_TLS_SKIP_VERIFY"),

		// Observability
		TracingEnabled:      envBool("OTEL_EXPORTER_OTLP_TRACES_ENABLED"),
		TracingExporter:     envString("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "console"),
		PromExporterEnabled: envBoolDefault("OTEL_PROM_EXPORTER_ENABLED", true),
		OtelMetricsEnabled:  envBool("OTEL_EXPORTER_OTLP_METRICS_ENABLED"),
		ProfilingAddr:       os.Getenv("PROFILING_BIND_ADDRESS"),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func envInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
	}
	return def
}

func envString(key, def string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return def
}

func envBool(key string) bool {
	return envBoolDefault(key, false)
}

func envBoolDefault(key string, def bool) bool {
	if s := os.Getenv(key); s != "" {
		if v, err := strconv.ParseBool(s); err == nil {
			return v
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if s := os.Getenv(key); s != "" {
		if d, ok := parseGoDuration(s); ok {
			return d
		}
	}
	return def
}

// parseGoDuration parses Go-style and k8s-style duration strings:
// "500ms", "1s", "20s", "1m", "1m30s", and also Go's time.ParseDuration format.
func parseGoDuration(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	// Try standard Go format first (handles most cases)
	if d, err := time.ParseDuration(s); err == nil {
		return d, true
	}
	return 0, false
}
