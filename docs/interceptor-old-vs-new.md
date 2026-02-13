# KEDA HTTP Add-on Interceptor: Old vs. New Implementation

This document describes the architectural differences between the original
Interceptor (`interceptor/`) and the new high-performance reimplementation
(`interceptor-new/`). Both expose identical external interfaces (same ports,
same env vars, same `/queue` JSON wire format) so the Scaler and Operator
work unchanged.

---

## Table of Contents

1. [Motivation](#motivation)
2. [At a Glance](#at-a-glance)
3. [Subsystem Comparison](#subsystem-comparison)
   - [Request Processing Pipeline](#request-processing-pipeline)
   - [Routing Table](#routing-table)
   - [Queue Counter](#queue-counter)
   - [Endpoints Cache and Cold-Start](#endpoints-cache-and-cold-start)
   - [Connection Pool and Backend Forwarding](#connection-pool-and-backend-forwarding)
   - [Buffer Management](#buffer-management)
   - [Metrics and Observability](#metrics-and-observability)
4. [File Layout Comparison](#file-layout-comparison)
5. [Startup and Lifecycle](#startup-and-lifecycle)
6. [Configuration](#configuration)
7. [Migration Guide](#migration-guide)

---

## Motivation

The original Go interceptor (`interceptor/`) was measured at around **1 k RPS**
under load testing. Profiling revealed several bottlenecks:

- A **single global `sync.RWMutex`** in the queue counter that every request
  contends on.
- **Two goroutines spawned per request** for the counting middleware
  (`countAsync` creates one goroutine; the deferred cleanup creates another).
- Use of **`httputil.ReverseProxy` + default `http.Transport`** settings,
  with no DNS caching and default pool sizes.
- A **layered middleware chain** with multiple `ResponseWriter` wrapper
  allocations per request.

The new Go interceptor (`interceptor-new/`) is a ground-up reimplementation
that eliminates every one of these bottlenecks. It replaces global locks with
per-host atomics, removes per-request goroutine spawning, uses a tuned
`httputil.ReverseProxy` with a custom `http.Transport` (DNS caching, optimised
pool settings), and collapses the middleware chain into a single flat
handler — targeting **100 k+ RPS** on the same hardware.

---

## At a Glance

| Aspect | Old (`interceptor/`) | New (`interceptor-new/`) |
|--------|---------------------|-------------------------|
| Routing table reads | `sync.RWMutex` around a radix tree | `atomic.Pointer` — lock-free |
| Queue counter hot path | Global `sync.RWMutex` | `sync.Map` + per-host `atomic.Int64` |
| Goroutines per request | 2 (counting middleware) | 0 (atomic ops + `defer`) |
| Backend forwarding | `httputil.ReverseProxy` + default `http.Transport` | `httputil.ReverseProxy` + custom `http.Transport` with DNS caching |
| Connection pool | `http.Transport` defaults | Tuned `http.Transport` (`MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout`) |
| DNS resolution | Per-connection via `http.Transport` | Cached in custom `DialContext` with configurable TTL (default 30 s) |
| Buffer allocations | Per-request (`bytes.Buffer`, `ResponseWriter` wrappers) | Zero wrappers — `httputil.ReverseProxy` manages buffers internally |
| Middleware chain | 4+ layers (Metrics → Logging → OTel → Routing → Counting → Forwarding) | Single flat handler function |
| Response streaming | `httputil.ReverseProxy` internal buffering | `httputil.ReverseProxy` streaming (same mechanism, tuned transport) |
| Endpoint cache | `client-go` informer + `watch.Broadcaster` per cold-start | `sync.Map` of `atomic.Int64` + close-and-replace broadcast channel |
| TLS termination | Same port, `ListenAndServeTLS` | Separate TLS port (8443), SNI routing, cert store support |
| OTel tracing | `otelhttp.NewHandler` middleware wrapper | Manual span creation in handler (zero cost when disabled) |
| Request logging | Middleware wrapper with stopwatch | Conditional log at handler exit (zero cost when disabled) |
| pprof profiling | Separate import | Dedicated server on configurable address |
| Named port resolution | Service lookup in forwarding handler | Service lookup at routing-table build time (pre-resolved) |

---

## Subsystem Comparison

### Request Processing Pipeline

#### Old implementation

Requests pass through a layered middleware chain. Each middleware wraps the
`http.ResponseWriter` with its own struct to capture the status code, bytes
written, or timing information:

```
Request ──▶ Metrics ──▶ [Logging] ──▶ [OTel] ──▶ Routing ──▶ Counting ──▶ Forwarding ──▶ Backend
```

Each layer allocates at least one wrapper struct on the heap. The Counting
middleware spawns two goroutines per request (one for the counter, one for
the signal).

#### New implementation

All logic runs in a single `ServeHTTP` method with no middleware wrapping
and no goroutine spawning:

```
ServeHTTP:
  1. Route lookup        ← atomic.Pointer load (zero allocation)
  2. Queue increase      ← atomic.Int64 add (zero allocation)
  3. Endpoint check      ← atomic.Int64 load (zero allocation, fast path)
  4. Forward request     ← httputil.ReverseProxy with custom Transport
  5. Record metrics      ← status captured via ModifyResponse callback
  6. Queue decrease      ← deferred guard.Release(), atomic CAS
```

The `httputil.ReverseProxy` handles connection pooling, header forwarding,
response streaming, and protocol compliance (HTTP/2, WebSocket upgrades,
trailers, etc.) via the custom `http.Transport` which adds DNS caching and
tuned pool settings.

### Routing Table

#### Old implementation (`pkg/routing/`)

- `Table` holds an `AtomicValue[*TableMemory]` (a `sync.Mutex`-based
  generic atomic wrapper).
- `TableMemory` is an **immutable radix tree**
  (`github.com/hashicorp/go-immutable-radix`).
- A `Signaler` triggers refresh from the informer event handler; the table's
  `Start()` method runs a goroutine that waits for signals and calls
  `refreshMemory`.
- `Route(req)` loads the table, calls `LongestPrefix`, then filters by
  headers.

#### New implementation (`routing.go`)

- `RoutingTable` holds an **`atomic.Pointer[tableMemory]`** (Go 1.19+
  standard library, genuinely lock-free — a single atomic load on the read
  path).
- `tableMemory` is a plain `map[string][]routeEntry` keyed by hostname.
  Entries are pre-sorted by path-prefix length (descending), then header
  count (descending).
- No radix tree dependency — the sorted-slice approach is simpler and
  performs comparably for the typical number of path prefixes per host
  (usually 1–3).
- **Pre-computed strings**: each `routeEntry` embeds a `RouteInfo` struct
  with `QueueKey`, `Authority`, `ServiceKey` etc., all computed once at
  build time. The old implementation computes some of these per request
  (e.g., `streamFromHTTPSO` builds the backend URL, `getPort` looks up the
  Service).
- Rebuild is triggered synchronously from the informer event handler (no
  separate signaler goroutine).

### Queue Counter

#### Old implementation (`pkg/queue/`)

```go
type Memory struct {
    concurrentMap map[string]int
    rpsMap        map[string]*RequestsBuckets
    mut           *sync.RWMutex          // ← single global lock
}
```

- **Every** `Increase` and `Decrease` call acquires the global write lock.
- At high concurrency, all CPU cores contend on this single mutex.
- `Current()` (called by `/queue`) acquires the read lock and iterates.

#### New implementation (`queue.go`)

```go
type QueueCounter struct {
    entries sync.Map                      // string -> *hostEntry
}

type hostEntry struct {
    concurrency atomic.Int64              // ← per-host, lock-free
    mu          sync.Mutex                // ← per-host, only for RPS buckets
    buckets     *rpsBuckets
}
```

- `Increase`: one `sync.Map` read (sharded internally) + one
  `atomic.Int64.Add`. The RPS recording takes a per-host mutex (not
  global).
- `Decrease`: one `sync.Map` read + one `atomic.Int64.CompareAndSwap` loop
  (clamped to zero).
- **No global lock on the hot path.** Different hosts never contend with
  each other.
- `QueueGuard` is an RAII-style struct that calls `Decrease` when
  `Release()` is called (via `defer`), replacing the old implementation's
  two-goroutine `countAsync` pattern.

### Endpoints Cache and Cold-Start

#### Old implementation (`pkg/k8s/`)

- `InformerBackedEndpointsCache` uses a `client-go` shared informer for
  `EndpointSlice` objects and a `watch.Broadcaster`.
- **Get path** (warm backend): uses the informer's Lister, which accesses
  the shared informer store (a `sync.Map`-backed `ThreadSafeStore`). Fast,
  but involves label-selector evaluation per call.
- **Watch path** (cold start): creates a `watch.Broadcaster` watcher with
  a goroutine and two channels per waiting request.

#### New implementation (`endpoints.go`)

- `EndpointsCache` maintains a derived `sync.Map` of
  `string -> *atomic.Int64` mapping service keys to ready-endpoint counts.
- **Fast path** (warm backend): **single `atomic.Int64.Load()`** — no label
  selector, no map iteration, no allocation.
- **Cold-start notification**: uses a close-and-replace channel broadcast
  pattern. When any endpoint count changes, the current `chan struct{}` is
  closed (waking all waiters) and replaced with a fresh one. Waiters select
  on the channel with a timeout. No per-waiter goroutine is created.

### Connection Pool and Backend Forwarding

#### Old implementation

Uses Go's standard `http.Transport`, which:

- Maintains a global pool of `*persistConn` per host.
- Each connection has a **background goroutine** (`readLoop`) that reads
  responses and dispatches them via channels.
- Checkout and return are channel-based operations with potential contention.
- `httputil.ReverseProxy` re-encodes request headers into the transport,
  allocates a `bufio.Writer`, and creates a new `http.Request`.
- DNS is resolved per connection via the `net.Dialer` (no caching).

#### New implementation (`transport.go`, `proxy.go`)

Uses `httputil.ReverseProxy` with a **custom `http.Transport`** that provides:

- **DNS caching** with configurable TTL (default 30 s) via a custom
  `DialContext`. Resolved addresses are stored in a `sync.Map` and reused
  until the TTL expires. Stable Kubernetes service names resolve to the same
  ClusterIP, so the cache is almost always a hit after the first request.
- **Tuned connection pool** via `MaxIdleConns`, `MaxIdleConnsPerHost`, and
  `IdleConnTimeout` — configured from environment variables.
- **TCP_NODELAY** is set on every new connection to reduce latency for
  small writes.
- **TLS client configuration** is derived from the proxy's inbound TLS
  config, so `https://` backend connections work correctly.

The `httputil.ReverseProxy` handles all protocol-level concerns:
- HTTP/1.1 and HTTP/2 support.
- WebSocket / connection upgrade support.
- Proper hop-by-hop header handling (RFC 7230).
- Trailer forwarding.
- `Expect: 100-continue`.
- `X-Forwarded-For` chaining (appends, not overwrites).
- Client disconnect propagation via context.

Status codes are captured via the `ModifyResponse` callback (no
`ResponseWriter` wrapper needed). Cold-start headers are injected in the
same callback.

### Buffer Management

#### Old implementation

- `httputil.ReverseProxy` uses a `sync.Pool` of 32 KB buffers (the
  `BufferPool` field), which is good.
- However, each middleware layer allocates its own `ResponseWriter` wrapper
  on the heap.
- `http.Transport` internally allocates a `bufio.Writer` per request.
- The counting middleware allocates channels and goroutines per request.

#### New implementation

- **`httputil.ReverseProxy`** manages its own internal buffer pool for
  response body copying.
- **No `ResponseWriter` wrappers** — the flat handler creates the
  `ReverseProxy` per request with `ModifyResponse` and `ErrorHandler`
  callbacks, avoiding any wrapper allocation.
- **No separate buffer pools** — the stdlib `ReverseProxy` and
  `http.Transport` handle all I/O buffering internally.

### Metrics and Observability

#### Old implementation (`interceptor/metrics/`)

- Two implementations: `PrometheusMetrics` and `OtelMetrics`.
- The Metrics middleware wraps `http.ResponseWriter` to capture the status
  code, then records after the request completes.
- Optional logging middleware starts a stopwatch and logs asynchronously.
- Optional OTel middleware wraps with `otelhttp.NewHandler`.

#### New implementation (`metrics.go`, `tracing.go`)

- Prometheus counters registered in a dedicated `prometheus.Registry`:
  - `interceptor_request_count_total{method, path, code, host}`
  - `interceptor_pending_request_count{host}`
- Recording is a direct method call at the end of `ServeHTTP` — no wrapper
  needed because the status code is captured via `ModifyResponse`.
- **OpenTelemetry tracing** (`OTEL_EXPORTER_OTLP_TRACES_ENABLED=true`)
  creates a per-request span directly in `ServeHTTP`:
  - Incoming `traceparent`/`b3` headers are extracted (W3C TraceContext +
    B3 propagation).
  - A server-side span is created with `http.method`, `http.target`,
    `http.host`, and `http.status_code` attributes.
  - Updated trace context is injected back into the request headers for
    propagation to the backend — no middleware wrapping, no extra
    allocations when tracing is disabled (the `tracer` field is nil).
  - Exporter is configurable via `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL`
    (`http/protobuf`, `grpc`, or `console`). Standard `OTEL_*` env vars
    control the endpoint, sampling, etc.
- **Request logging** (`KEDA_HTTP_LOG_REQUESTS=true`) emits a structured
  log line per request with method, path, host, status code, and
  duration — written at the same point as the metric recording, so there
  is zero overhead when disabled.

### TLS Termination

#### Old implementation (`tls_config.go`)

- Full TLS support with SNI-based certificate selection, cert store
  directories, and system CA pool loading.
- Enabled via `KEDA_HTTP_PROXY_TLS_ENABLED`.

#### New implementation (`tls_config.go`)

- Same capability: primary cert/key pair, cert store directories, SNI
  matching against x509 SANs, system CA pool.
- Uses the same `logr` + `zap` logging as the operator and scaler.
- The TLS listener runs on a **separate port** (`KEDA_HTTP_PROXY_TLS_PORT`,
  default 8443), allowing a gradual migration where some clients use TLS
  and others continue over plain HTTP.
- `X-Forwarded-Proto` is set correctly to `https` for TLS connections.

### Profiling

#### Old implementation

- pprof is started via a separate import and `net/http/pprof` registration.

#### New implementation

- pprof is available when `PROFILING_BIND_ADDRESS` is set (e.g. `:6060`).
  A dedicated `http.Server` on that address serves the standard Go pprof
  handlers. When the env var is empty, no profiling server is started and
  there is zero performance impact.

---

## File Layout Comparison

### Old implementation

```
interceptor/
├── main.go                     Entry point, errgroup lifecycle
├── proxy.go                    BuildProxyHandler (middleware chain assembly)
├── proxy_handlers.go           newForwardingHandler (backend wait + proxy)
├── forward_wait_func.go        forwardWaitFunc (cold-start wait logic)
├── admin.go                    BuildAdminHandler (/livez, /readyz, /queue)
├── tls_config.go               TLS setup
├── config/                     Configuration structs
├── handler/                    Upstream (reverse proxy), Probe (health), Static
├── middleware/                  Routing, Counting, Metrics, Logging
├── metrics/                    Prometheus + OTel metric collectors
└── tracing/                    OTel tracing setup

pkg/
├── queue/                      In-memory queue counter (global RWMutex)
├── routing/                    Routing table (radix tree + AtomicValue)
├── k8s/                        EndpointsCache (informer + Broadcaster)
├── http/                       TransportPool (pools http.Transport by timeout)
├── net/                        Retry dialer with backoff
└── util/                       AtomicValue, Signaler, context helpers
```

### New implementation

```
interceptor-new/
├── main.go          Entry point, K8s informer setup, errgroup lifecycle
├── config.go        Configuration (flat struct, env parsing)
├── routing.go       Lock-free routing table (atomic.Pointer) + named port resolution
├── queue.go         Atomic queue counter + RPS ring buffer (sync.Map)
├── endpoints.go     EndpointSlice cache (sync.Map + broadcast channel)
├── transport.go     Custom http.Transport with DNS cache
├── proxy.go         httputil.ReverseProxy handler (the hot path) + request logging
├── admin.go         Admin server (/livez, /readyz, /queue, /debug/stats)
├── metrics.go       Prometheus metrics
├── tls_config.go    TLS certificate loading + SNI routing
├── tracing.go       OpenTelemetry tracing setup
└── Dockerfile       Container build
```

All code is in a single Go package (`main`) — no sub-packages, no import
cycles, no interface indirection on the hot path.

---

## Startup and Lifecycle

Both implementations use the same pattern:

1. Parse configuration from environment variables.
2. Create Kubernetes clients.
3. Set up informers for `HTTPScaledObject` and `EndpointSlice`.
4. Create subsystems (routing table, queue counter, endpoints cache).
5. Start all servers via `errgroup.WithContext`.
6. Shut down gracefully on context cancellation or fatal error.

**Differences:**

- The old implementation uses a full `ctrl.NewManager()` from
  controller-runtime, which brings in leader election, webhooks, and other
  machinery not needed by the interceptor. The new implementation uses
  `cache.New()` directly — lighter weight, same informer functionality.
- The old routing table has its own `Start()` goroutine that listens on a
  `Signaler` channel. The new routing table rebuilds synchronously inside
  the informer event handler (an atomic swap is fast enough).

---

## Configuration

Both implementations read the same environment variables with the same
defaults. The new implementation uses identical env var names to ensure
drop-in compatibility.

### Shared variables (same name and default in both implementations)

| Variable | Default | Purpose |
|----------|---------|---------|
| `KEDA_HTTP_PROXY_PORT` | `8080` | Proxy server listen port |
| `KEDA_HTTP_ADMIN_PORT` | `9090` | Admin server listen port |
| `OTEL_PROM_EXPORTER_PORT` | `2223` | Prometheus metrics port |
| `KEDA_HTTP_CONNECT_TIMEOUT` | `500ms` | TCP connect timeout to backends |
| `KEDA_HTTP_KEEP_ALIVE` | `1s` | TCP keep-alive interval |
| `KEDA_RESPONSE_HEADER_TIMEOUT` | `500ms` | Wait for backend response headers |
| `KEDA_CONDITION_WAIT_TIMEOUT` | `20s` | Max wait for cold-start scale-up |
| `KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT` | `10s` | TLS handshake timeout to backends |
| `KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT` | `1s` | Timeout for `Expect: 100-continue` responses |
| `KEDA_HTTP_FORCE_HTTP2` | `false` | Force HTTP/2 to backends |
| `KEDA_HTTP_MAX_IDLE_CONNS` | `100` | Max idle connections across all hosts |
| `KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST` | `20` | Max idle connections per backend host |
| `KEDA_HTTP_IDLE_CONN_TIMEOUT` | `90s` | Close idle connections after this duration |
| `KEDA_HTTP_WATCH_NAMESPACE` | (all) | Namespace filter for HTTPScaledObjects |
| `KEDA_HTTP_LOG_REQUESTS` | `false` | Enable request logging |
| `KEDA_HTTP_ENABLE_COLD_START_HEADER` | `true` | Send `X-KEDA-HTTP-Cold-Start` header |
| `KEDA_HTTP_PROXY_TLS_ENABLED` | `false` | Enable TLS on proxy port |
| `KEDA_HTTP_PROXY_TLS_PORT` | `8443` | TLS proxy listen port (when TLS enabled) |
| `KEDA_HTTP_PROXY_TLS_CERT_PATH` | `/certs/tls.crt` | Path to primary TLS certificate |
| `KEDA_HTTP_PROXY_TLS_KEY_PATH` | `/certs/tls.key` | Path to primary TLS private key |
| `KEDA_HTTP_PROXY_TLS_CERT_STORE_PATHS` | (none) | Comma-separated cert store directories |
| `KEDA_HTTP_PROXY_TLS_SKIP_VERIFY` | `false` | Skip TLS verification for upstreams |
| `OTEL_PROM_EXPORTER_ENABLED` | `true` | Enable Prometheus metrics endpoint |
| `OTEL_EXPORTER_OTLP_TRACES_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` | `console` | Tracing exporter (`http/protobuf`, `grpc`, `console`) |
| `OTEL_EXPORTER_OTLP_METRICS_ENABLED` | `false` | Enable OTel OTLP metrics export (alongside Prometheus) |
| `PROFILING_BIND_ADDRESS` | (none) | pprof server address (e.g. `:6060`); empty = disabled |

### New variables (only in the new implementation)

| Variable | Default | Purpose |
|----------|---------|---------|
| `KEDA_HTTP_DNS_CACHE_TTL` | `30s` | DNS cache TTL for backend resolution |

### Old variables not carried over

| Variable | Reason |
|----------|--------|
| `KEDA_HTTP_CURRENT_NAMESPACE` | Defined but unused by the old interceptor |
| `KEDA_HTTP_DIAL_RETRY_TIMEOUT` | Cold-start probe replaces the dial-retry logic |
| `KEDA_HTTP_SCALER_CONFIG_MAP_INFORMER_RSYNC_PERIOD` | Different cache setup |
| `KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS` | Different architecture (event-driven, not polling) |

---

## Migration Guide

The new interceptor is a **drop-in replacement**. To switch:

1. **Build** the new binary: `go build -o interceptor ./interceptor-new/`
2. **Update** the Dockerfile reference (or use `interceptor-new/Dockerfile`).
3. **Deploy** with the same environment variables — no changes needed.
4. **Verify**:
   - `/livez` and `/readyz` return 200 once the routing table syncs.
   - `/queue` returns the same JSON format the Scaler expects.
   - `/metrics` exposes `interceptor_request_count_total` and
     `interceptor_pending_request_count`.

**Feature parity achieved.** All features from the old implementation have
been ported:

- **TLS termination** — `KEDA_HTTP_PROXY_TLS_ENABLED`, separate TLS port,
  SNI routing, cert store directories, `KEDA_HTTP_PROXY_TLS_SKIP_VERIFY`.
- **OpenTelemetry tracing** — `OTEL_EXPORTER_OTLP_TRACES_ENABLED`, W3C
  TraceContext + B3 propagation, configurable exporter.
- **Request logging** — `KEDA_HTTP_LOG_REQUESTS`, structured logging via
  `logr`/`zap` (matching operator and scaler) with method, path, host,
  status, and duration.
- **pprof profiling** — `PROFILING_BIND_ADDRESS` starts a dedicated
  profiling server.
- **Named port resolution** — `spec.scaleTargetRef.portName` is resolved
  via Service lookup at routing-table build time.
- **Cold-start header** — `KEDA_HTTP_ENABLE_COLD_START_HEADER` controls
  the `X-KEDA-HTTP-Cold-Start` response header (default: enabled).
- **Connection pool tuning** — `KEDA_HTTP_MAX_IDLE_CONNS`,
  `KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST`, and `KEDA_HTTP_IDLE_CONN_TIMEOUT`.
