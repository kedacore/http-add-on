// KEDA HTTP Add-on Interceptor — high-performance Go reimplementation.
//
// A reverse proxy that:
//  1. Matches incoming requests to HTTPScaledObject routing rules (lock-free).
//  2. Tracks per-host concurrency & RPS for KEDA autoscaling (atomic ops, no global lock).
//  3. Forwards via httputil.ReverseProxy with a custom http.Transport (DNS caching).
//  4. Exposes Prometheus metrics compatible with the existing scaler.
//
// Key performance improvements over the original Go interceptor:
//   - Lock-free routing table via atomic.Pointer (was: radix tree behind RWMutex)
//   - Per-host atomic counters for queue (was: single global RWMutex)
//   - httputil.ReverseProxy with tuned Transport + DNS caching (was: default pool)
//   - Zero per-request goroutine spawning for counting (was: 2 goroutines/request)
//   - Single flat handler (was: 4+ middleware wrappers per request)
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("interceptor")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = httpv1alpha1.AddToScheme(scheme)
}

func main() {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if err := run(); err != nil {
		setupLog.Error(err, "Fatal error")
		os.Exit(1)
	}
}

func run() error {
	cfg := ConfigFromEnv()
	setupLog.Info("Starting interceptor-new",
		"proxy_port", cfg.ProxyPort,
		"admin_port", cfg.AdminPort,
		"metrics_port", cfg.MetricsPort,
	)

	// ---- Kubernetes clients ------------------------------------------------
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes config: %w", err)
	}
	// Increase QPS for the controller-runtime cache
	restConfig.QPS = 100
	restConfig.Burst = 200

	// ---- OpenTelemetry tracing (optional) ------------------------------------
	var tracer trace.Tracer
	if cfg.TracingEnabled {
		t, shutdown, err := setupTracing(context.Background(), cfg.TracingExporter)
		if err != nil {
			return fmt.Errorf("failed to set up tracing: %w", err)
		}
		defer func() { _ = shutdown(context.Background()) }()
		tracer = t
		setupLog.Info("OpenTelemetry tracing enabled", "exporter", cfg.TracingExporter)
	}

	// ---- Subsystems --------------------------------------------------------
	routingTable := NewRoutingTable()
	queueCounter := NewQueueCounter()
	endpointsCache := NewEndpointsCache()
	transportStats := &TransportStats{}
	transport := NewTransport(&TransportConfig{
		ConnectTimeout:        cfg.ConnectTimeout,
		KeepAlive:             cfg.KeepAlive,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		DNSCacheTTL:           cfg.DNSCacheTTL,
		ForceHTTP2:            cfg.ForceHTTP2,
	}, transportStats)
	metricsCollector := NewMetricsCollector(cfg.OtelMetricsEnabled)
	defer metricsCollector.Shutdown(context.Background())

	// ---- Context with signal handling --------------------------------------
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// ---- Start controller-runtime cache (HTTPScaledObject watcher) ---------
	cacheOpts := cache.Options{
		Scheme: scheme,
	}
	if cfg.WatchNamespace != "" {
		cacheOpts.DefaultNamespaces = map[string]cache.Config{
			cfg.WatchNamespace: {},
		}
	}

	ctrlCache, err := cache.New(restConfig, cacheOpts)
	if err != nil {
		return fmt.Errorf("failed to create controller-runtime cache: %w", err)
	}

	// Register the informer event handler for HTTPScaledObject changes.
	// On any add/update/delete, rebuild the routing table.
	httpsoInformer, err := ctrlCache.GetInformer(ctx, &httpv1alpha1.HTTPScaledObject{})
	if err != nil {
		return fmt.Errorf("failed to get HTTPScaledObject informer: %w", err)
	}

	reader := ctrlCache

	_, _ = httpsoInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			rebuildRoutingTable(ctx, reader, routingTable, queueCounter, cfg.WatchNamespace)
		},
		UpdateFunc: func(_, _ interface{}) {
			rebuildRoutingTable(ctx, reader, routingTable, queueCounter, cfg.WatchNamespace)
		},
		DeleteFunc: func(_ interface{}) {
			rebuildRoutingTable(ctx, reader, routingTable, queueCounter, cfg.WatchNamespace)
		},
	})

	// ---- Start EndpointSlice informer ------------------------------------
	epsInformer, err := ctrlCache.GetInformer(ctx, &discoveryv1.EndpointSlice{})
	if err != nil {
		return fmt.Errorf("failed to get EndpointSlice informer: %w", err)
	}

	_, _ = epsInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handleEndpointSliceEvent(ctx, reader, endpointsCache, obj)
		},
		UpdateFunc: func(_, obj interface{}) {
			handleEndpointSliceEvent(ctx, reader, endpointsCache, obj)
		},
		DeleteFunc: func(obj interface{}) {
			handleEndpointSliceEvent(ctx, reader, endpointsCache, obj)
		},
	})

	// Start the controller-runtime cache
	g.Go(func() error {
		setupLog.Info("Starting controller-runtime cache")
		return ctrlCache.Start(ctx)
	})

	// Wait for caches to sync before starting servers
	g.Go(func() error {
		if !ctrlCache.WaitForCacheSync(ctx) {
			return fmt.Errorf("failed to sync controller-runtime caches")
		}
		setupLog.Info("Controller-runtime caches synced")

		// Trigger an initial routing table build
		rebuildRoutingTable(ctx, reader, routingTable, queueCounter, cfg.WatchNamespace)
		return nil
	})

	// ---- Start servers -----------------------------------------------------
	var proxyOpts []ProxyOption
	if tracer != nil {
		proxyOpts = append(proxyOpts, WithTracer(tracer))
	}

	// TLS configuration (optional)
	if cfg.TLSEnabled {
		tlsCfg, err := BuildTLSConfig(&cfg)
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		proxyOpts = append(proxyOpts, WithTLSConfig(tlsCfg))

		// Configure the transport for TLS connections to backends.
		// Mirrors the inbound TLS config so that backend HTTPS works.
		transport.TLSClientConfig = &tls.Config{
			RootCAs:            tlsCfg.RootCAs,
			Certificates:       tlsCfg.Certificates,
			InsecureSkipVerify: tlsCfg.InsecureSkipVerify, //nolint:gosec // mirrors inbound config
		}

		setupLog.Info("TLS enabled for proxy", "tls_port", cfg.TLSProxyPort)
	}

	proxyServer := NewProxyServer(&cfg, routingTable, queueCounter, endpointsCache, transport, metricsCollector, proxyOpts...)
	adminServer := NewAdminServer(&cfg, routingTable, queueCounter, transportStats)
	metricsServer := NewMetricsServer(&cfg, metricsCollector)

	g.Go(func() error { return proxyServer.ListenAndServe() })
	g.Go(func() error { return adminServer.ListenAndServe() })
	if cfg.PromExporterEnabled {
		g.Go(func() error { return metricsServer.ListenAndServe() })
	}

	// TLS proxy server (optional, separate port)
	if cfg.TLSEnabled {
		g.Go(func() error { return proxyServer.ListenAndServeTLS() })
	}

	// pprof profiling server (optional)
	if cfg.ProfilingAddr != "" {
		pprofAddr := cfg.ProfilingAddr
		g.Go(func() error {
			setupLog.Info("pprof profiling server listening", "addr", pprofAddr)
			return http.ListenAndServe(pprofAddr, nil) // DefaultServeMux has pprof routes
		})
	}

	// ---- Wait for shutdown or fatal error ----------------------------------
	if err := g.Wait(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("shutting down due to error: %w", err)
	}
	setupLog.Info("Shutting down")
	return nil
}

// ---------------------------------------------------------------------------
// Routing table rebuild
// ---------------------------------------------------------------------------

func rebuildRoutingTable(
	ctx context.Context,
	reader client.Reader,
	rt *RoutingTable,
	q *QueueCounter,
	watchNamespace string,
) {
	var httpsoList httpv1alpha1.HTTPScaledObjectList
	opts := &client.ListOptions{}
	if watchNamespace != "" {
		opts.Namespace = watchNamespace
	}

	if err := reader.List(ctx, &httpsoList, opts); err != nil {
		setupLog.Error(err, "failed to list HTTPScaledObjects")
		return
	}

	// Build a port resolver that looks up Service objects via the cache
	resolvePort := newPortResolver(ctx, reader)

	// Rebuild routing table (atomic swap)
	rt.Rebuild(httpsoList.Items, resolvePort)

	// Sync queue keys
	syncQueueKeys(q, httpsoList.Items)

	setupLog.V(1).Info("Routing table rebuilt", "count", len(httpsoList.Items))
}

// newPortResolver returns a PortResolver that looks up a Service and finds
// the numeric port matching the given portName.
func newPortResolver(ctx context.Context, reader client.Reader) PortResolver {
	return func(namespace, service, portName string) int32 {
		var svc corev1.Service
		key := types.NamespacedName{Namespace: namespace, Name: service}
		if err := reader.Get(ctx, key, &svc); err != nil {
			setupLog.Info("Failed to resolve named port: Service lookup failed",
				"namespace", namespace,
				"service", service,
				"portName", portName,
				"error", err,
			)
			return 0
		}
		for _, p := range svc.Spec.Ports {
			if p.Name == portName {
				return p.Port
			}
		}
		setupLog.Info("Named port not found in Service",
			"namespace", namespace,
			"service", service,
			"portName", portName,
		)
		return 0
	}
}

func syncQueueKeys(q *QueueCounter, objects []httpv1alpha1.HTTPScaledObject) {
	newKeys := make(map[string]struct{}, len(objects))

	for i := range objects {
		httpso := &objects[i]
		namespace := httpso.Namespace
		if namespace == "" {
			namespace = defaultNamespace
		}
		key := namespace + "/" + httpso.Name
		q.EnsureKey(key)
		newKeys[key] = struct{}{}

		// Configure RPS buckets if requestRate metric is defined
		if sm := httpso.Spec.ScalingMetric; sm != nil {
			if rr := sm.Rate; rr != nil {
				window := rr.Window.Duration
				granularity := rr.Granularity.Duration
				if window <= 0 {
					window = time.Minute
				}
				if granularity <= 0 {
					granularity = time.Second
				}
				q.UpdateBuckets(key, window, granularity)
			}
		}
	}

	q.RetainKeys(newKeys)
}

// ---------------------------------------------------------------------------
// EndpointSlice event handler
// ---------------------------------------------------------------------------

func handleEndpointSliceEvent(
	ctx context.Context,
	reader client.Reader,
	ec *EndpointsCache,
	obj interface{},
) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		// Handle deleted objects wrapped in DeletedFinalStateUnknown
		if d, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
			slice, ok = d.Obj.(*discoveryv1.EndpointSlice)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	namespace, svcName, ok := extractServiceFromSlice(slice)
	if !ok {
		return
	}

	// List all EndpointSlices for this service
	var sliceList discoveryv1.EndpointSliceList
	if err := reader.List(ctx, &sliceList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(labels.Set{
				"kubernetes.io/service-name": svcName,
			}),
		},
	); err != nil {
		setupLog.Error(err, "failed to list EndpointSlices", "service", svcName)
		return
	}

	// Convert to pointer slice
	ptrs := make([]*discoveryv1.EndpointSlice, len(sliceList.Items))
	for i := range sliceList.Items {
		ptrs[i] = &sliceList.Items[i]
	}

	ec.UpdateService(namespace, svcName, ptrs)
}
