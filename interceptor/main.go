package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: pprof intentionally exposed, gated by --profiling-addr
	"os"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/build"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/observability"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var setupLog = ctrl.Log.WithName("setup")

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func main() {
	if err := run(); err != nil {
		setupLog.Error(err, "fatal error")
		os.Exit(1)
	}
}

func run() error {
	timeoutCfg := config.MustParseTimeouts(setupLog)
	servingCfg := config.MustParseServing()
	metricsCfg := observability.MustParseMetricsConfig()
	tracingCfg := observability.MustParseTracingConfig()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info(
		"starting interceptor",
		"timeoutConfig", timeoutCfg,
		"servingConfig", servingCfg,
		"metricsConfig", metricsCfg,
	)

	provider, err := observability.NewMeterProvider(metrics.ServiceName, metricsCfg)
	if err != nil {
		return fmt.Errorf("creating meter provider: %w", err)
	}
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			setupLog.Error(err, "error shutting down meter provider")
		}
	}()

	instruments, err := metrics.NewInstruments(provider)
	if err != nil {
		return fmt.Errorf("creating metric instruments: %w", err)
	}

	cfg := ctrl.GetConfigOrDie()
	signalCtx := ctrl.SetupSignalHandler()

	cacheOpts := cache.Options{
		DefaultTransform: cache.TransformStripManagedFields(),
		Scheme:           kedacache.NewScheme(),
		SyncPeriod:       &servingCfg.CacheSyncPeriod,
		// Scope the ConfigMap informer to only cache labeled ConfigMaps,
		// avoiding cluster-wide caching of all ConfigMaps on first Get.
		ByObject: map[client.Object]cache.ByObject{
			&corev1.ConfigMap{}: {
				Label: k8s.ResponseBodyLabels.AsSelector(),
			},
		},
	}
	if servingCfg.WatchNamespace != "" {
		cacheOpts.DefaultNamespaces = map[string]cache.Config{
			servingCfg.WatchNamespace: {},
		}
	}

	ctrlCache, err := cache.New(cfg, cacheOpts)
	if err != nil {
		return fmt.Errorf("creating cache: %w", err)
	}

	readyCache, err := k8s.NewReadyEndpointsCacheWithInformer(signalCtx, ctrl.Log, ctrlCache)
	if err != nil {
		return fmt.Errorf("creating endpoints cache: %w", err)
	}

	queues := queue.NewMemory()
	routingTable := routing.NewTable(ctrlCache, queues)

	// Setup informers to signal routing table on IR and HTTPSO changes
	routeSources := []client.Object{&v1beta1.InterceptorRoute{}, &v1alpha1.HTTPScaledObject{}}
	for _, obj := range routeSources {
		informer, err := ctrlCache.GetInformer(signalCtx, obj)
		if err != nil {
			return fmt.Errorf("getting informer for %T: %w", obj, err)
		}

		_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc:    func(_ any) { routingTable.Signal() },
			UpdateFunc: func(_, _ any) { routingTable.Signal() },
			DeleteFunc: func(_ any) { routingTable.Signal() },
		})
		if err != nil {
			return fmt.Errorf("adding event handlers: %w", err)
		}
	}

	if tracingCfg.Enabled {
		shutdown, err := tracing.SetupOTelSDK(signalCtx, tracingCfg)
		if err != nil {
			return fmt.Errorf("setting up tracer: %w", err)
		}
		defer func() {
			if shutdownErr := shutdown(context.Background()); shutdownErr != nil {
				setupLog.Error(shutdownErr, "error during tracer shutdown")
			}
		}()
	}

	var draining atomic.Bool
	probeHandler := handler.NewProbe(&draining, routingTable)

	// Two separate lifecycles: proxy servers drain before infrastructure shuts
	// down. This keeps the admin server (readiness probe) alive during proxy
	// drain so Kubernetes sees 503 rather than connection refused.
	proxyCtx, cancelProxy := context.WithCancel(context.Background())
	defer cancelProxy()
	proxyCtx = util.ContextWithLogger(proxyCtx, ctrl.Log)

	infraCtx, cancelInfra := context.WithCancel(context.Background())
	defer cancelInfra()
	infraCtx = util.ContextWithLogger(infraCtx, ctrl.Log)

	proxyEg, proxyCtx := errgroup.WithContext(proxyCtx)
	infraEg, infraCtx := errgroup.WithContext(infraCtx)

	// If infrastructure dies, bring down proxy too.
	context.AfterFunc(infraCtx, cancelProxy)

	// --- Infrastructure goroutines (use infraCtx) ---

	infraEg.Go(func() error {
		setupLog.Info("starting the controller-runtime cache")
		return ctrlCache.Start(infraCtx)
	})

	// Wait for cache to sync before starting components that depend on it
	if !ctrlCache.WaitForCacheSync(infraCtx) {
		return fmt.Errorf("cache failed to sync")
	}

	// Start the update loop that refreshes the routing table on InterceptorRoute & HTTPSO changes
	infraEg.Go(func() error {
		setupLog.Info("starting the routing table")
		if err := routingTable.Start(infraCtx); !util.IsIgnoredErr(err) {
			return fmt.Errorf("routing table: %w", err)
		}
		return nil
	})

	infraEg.Go(func() error {
		probeHandler.Start(infraCtx)
		return nil
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	infraEg.Go(func() error {
		setupLog.Info("starting the admin server", "port", servingCfg.AdminPort)
		if err := runAdminServer(infraCtx, ctrl.Log, servingCfg.AdminPort, queues, probeHandler); !util.IsIgnoredErr(err) {
			return fmt.Errorf("admin server: %w", err)
		}
		return nil
	})

	if metricsCfg.OtelPrometheusExporterEnabled {
		// start the prometheus compatible metrics server
		// serves a prometheus compatible metrics endpoint on the configured port
		infraEg.Go(func() error {
			if err := runMetricsServer(infraCtx, ctrl.Log, metricsCfg); !util.IsIgnoredErr(err) {
				return fmt.Errorf("metrics server: %w", err)
			}
			return nil
		})
	}

	if servingCfg.ProfilingAddr != "" {
		infraEg.Go(func() error {
			setupLog.Info("enabling pprof for profiling", "address", servingCfg.ProfilingAddr)
			if err := kedahttp.ServeContext(infraCtx, kedahttp.ServerConfig{
				Addr:    servingCfg.ProfilingAddr,
				Handler: http.DefaultServeMux,
			}); !util.IsIgnoredErr(err) {
				return fmt.Errorf("pprof server: %w", err)
			}
			return nil
		})
	}

	// --- Proxy goroutines (use proxyCtx) ---

	// start the proxy servers. This is the server that
	// accepts, holds and forwards user requests
	if servingCfg.ProxyTLSEnabled {
		proxyEg.Go(func() error {
			tlsCfg, err := BuildTLSConfig(TLSOptions{
				CertificatePath:    servingCfg.TLSCertPath,
				KeyPath:            servingCfg.TLSKeyPath,
				CertStorePaths:     servingCfg.TLSCertStorePaths,
				InsecureSkipVerify: servingCfg.TLSSkipVerify,
				MinTLSVersion:      servingCfg.TLSMinVersion,
				MaxTLSVersion:      servingCfg.TLSMaxVersion,
				CipherSuites:       servingCfg.TLSCipherSuites,
				CurvePreferences:   servingCfg.TLSCurvePreferences,
			}, setupLog)
			if err != nil {
				return fmt.Errorf("configuring TLS: %w", err)
			}

			setupLog.Info("starting the proxy server with TLS enabled", "port", servingCfg.TLSPort)
			if err := runProxyServer(proxyCtx, ctrl.Log, queues, readyCache, routingTable, ctrlCache, timeoutCfg, servingCfg, servingCfg.TLSPort, tlsCfg, tracingCfg, instruments, &draining); !util.IsIgnoredErr(err) {
				return fmt.Errorf("tls proxy server: %w", err)
			}
			return nil
		})
	}

	proxyEg.Go(func() error {
		setupLog.Info("starting the proxy server", "port", servingCfg.ProxyPort)
		if err := runProxyServer(proxyCtx, ctrl.Log, queues, readyCache, routingTable, ctrlCache, timeoutCfg, servingCfg, servingCfg.ProxyPort, nil, tracingCfg, instruments, &draining); !util.IsIgnoredErr(err) {
			return fmt.Errorf("proxy server: %w", err)
		}
		return nil
	})

	// --- Shutdown sequencing ---
	//
	// Pod deletion triggers the EndpointSlice removal and a SIGTERM:
	//   1. Mark readiness probe unhealthy (503 on /readyz) for external load balancers
	//   2. Delay the Shutdown e.g. to allow endpoint removal to be propagated to all nodes
	//   3. Close proxy listener, drain in-flight requests
	//   4. After proxy drain completes, stop infrastructure
	go func() {
		<-signalCtx.Done()

		setupLog.Info("shutdown: starting drain")
		draining.Store(true)

		if servingCfg.ShutdownDelay > 0 {
			setupLog.Info("shutdown: delaying shutdown while still accepting new connections", "delay", servingCfg.ShutdownDelay)
			time.Sleep(servingCfg.ShutdownDelay)
		}

		setupLog.Info("shutdown: draining proxy connections", "timeout", servingCfg.DrainTimeout)
		cancelProxy()
	}()

	build.PrintComponentInfo(ctrl.Log, "Interceptor")
	setupLog.Info("Interceptor starting")

	// Wait for proxy to finish (blocks during normal operation).
	proxyErr := proxyEg.Wait()

	// Proxy drained (or failed), shut down infrastructure.
	setupLog.Info("shutdown: proxy drained, stopping infrastructure")
	cancelInfra()
	infraErr := infraEg.Wait()

	if proxyErr != nil && !util.IsIgnoredErr(proxyErr) {
		return fmt.Errorf("proxy: %w", proxyErr)
	}
	if infraErr != nil && !util.IsIgnoredErr(infraErr) {
		return fmt.Errorf("infrastructure: %w", infraErr)
	}

	setupLog.Info("Bye!")
	return nil
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	q queue.Counter,
	probeHandler *handler.Probe,
) error {
	lggr = lggr.WithName("runAdminServer")

	adminHandler := BuildAdminHandler(lggr, q, probeHandler)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return kedahttp.ServeContext(ctx, kedahttp.ServerConfig{
		Addr:    addr,
		Handler: adminHandler,
	})
}

func runMetricsServer(
	ctx context.Context,
	lggr logr.Logger,
	metricsCfg config.Metrics,
) error {
	lggr.Info("starting the prometheus metrics server", "port", metricsCfg.OtelPrometheusExporterPort, "path", "/metrics")
	addr := fmt.Sprintf("0.0.0.0:%d", metricsCfg.OtelPrometheusExporterPort)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return kedahttp.ServeContext(ctx, kedahttp.ServerConfig{
		Addr:    addr,
		Handler: mux,
	})
}

func runProxyServer(
	ctx context.Context,
	logger logr.Logger,
	q queue.Counter,
	readyCache *k8s.ReadyEndpointsCache,
	routingTable routing.Table,
	reader client.Reader,
	timeouts config.Timeouts,
	serving config.Serving,
	port int,
	tlsCfg *tls.Config,
	tracingConfig config.Tracing,
	instruments *metrics.Instruments,
	draining *atomic.Bool,
) error {
	// Build handler chain using the shared builder
	rootHandler := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       logger,
		Queue:        q,
		ReadyCache:   readyCache,
		RoutingTable: routingTable,
		Reader:       reader,
		Timeouts:     timeouts,
		Serving:      serving,
		TLSConfig:    tlsCfg,
		Tracing:      tracingConfig,
		Instruments:  instruments,
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info("proxy server starting", "address", addr)
	return kedahttp.ServeContext(ctx, kedahttp.ServerConfig{
		Addr:         addr,
		Handler:      rootHandler,
		TLSConfig:    tlsCfg,
		DrainTimeout: serving.DrainTimeout,
		Draining:     draining,
	})
}
