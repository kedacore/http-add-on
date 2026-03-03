package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: pprof intentionally exposed, gated by --profiling-addr
	"os"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
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
	"github.com/kedacore/http-add-on/pkg/build"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var setupLog = ctrl.Log.WithName("setup")

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func main() {
	defer os.Exit(1)
	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()
	metricsCfg := config.MustParseMetrics()
	tracingCfg := config.MustParseTracing()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if err := config.Validate(servingCfg, timeoutCfg); err != nil {
		setupLog.Error(err, "invalid configuration")
		runtime.Goexit()
	}

	setupLog.Info(
		"starting interceptor",
		"timeoutConfig",
		timeoutCfg,
		"servingConfig",
		servingCfg,
		"metricsConfig",
		metricsCfg,
	)

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort
	proxyTLSEnabled := servingCfg.ProxyTLSEnabled
	profilingAddr := servingCfg.ProfilingAddr

	// setup the configured metrics collectors
	metrics.NewMetricsCollectors(metricsCfg)

	cfg := ctrl.GetConfigOrDie()

	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "creating new Kubernetes ClientSet")
		runtime.Goexit()
	}

	endpointsCache := k8s.NewInformerBackedEndpointsCache(ctrl.Log, cl, time.Millisecond*time.Duration(servingCfg.EndpointsCachePollIntervalMS))
	waitFunc := newWorkloadReplicasForwardWaitFunc(endpointsCache.ReadyCache())

	cacheOpts := cache.Options{
		Scheme:     kedacache.NewScheme(),
		SyncPeriod: &servingCfg.CacheSyncPeriod,
	}
	if servingCfg.WatchNamespace != "" {
		cacheOpts.DefaultNamespaces = map[string]cache.Config{
			servingCfg.WatchNamespace: {},
		}
	}

	ctrlCache, err := cache.New(cfg, cacheOpts)
	if err != nil {
		setupLog.Error(err, "creating cache")
		runtime.Goexit()
	}

	queues := queue.NewMemory()

	routingTable := routing.NewTable(ctrlCache, queues)

	// Setup informer to signal routing table on HTTPSO changes
	informer, err := ctrlCache.GetInformer(context.Background(), &v1alpha1.HTTPScaledObject{})
	if err != nil {
		setupLog.Error(err, "getting HTTPScaledObject informer")
		runtime.Goexit()
	}

	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { routingTable.Signal() },
		UpdateFunc: func(_, _ any) { routingTable.Signal() },
		DeleteFunc: func(_ any) { routingTable.Signal() },
	})
	if err != nil {
		setupLog.Error(err, "adding event handlers")
		runtime.Goexit()
	}

	setupLog.Info("Interceptor starting")

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, ctrl.Log)

	eg, ctx := errgroup.WithContext(ctx)

	if tracingCfg.Enabled {
		shutdown, err := tracing.SetupOTelSDK(ctx, tracingCfg)
		if err != nil {
			setupLog.Error(err, "Error setting up tracer")
			runtime.Goexit()
		}

		defer func() {
			if shutdownErr := shutdown(context.Background()); shutdownErr != nil {
				setupLog.Error(shutdownErr, "Error during tracer shutdown")
			}
		}()
	}

	eg.Go(func() error {
		setupLog.Info("starting the controller-runtime cache")
		return ctrlCache.Start(ctx)
	})

	// Wait for cache to sync before starting components that depend on it
	if !ctrlCache.WaitForCacheSync(ctx) {
		setupLog.Error(nil, "cache failed to sync")
		runtime.Goexit()
	}

	// start the endpoints cache updater
	eg.Go(func() error {
		setupLog.Info("starting the endpoints cache")

		endpointsCache.Start(ctx)
		return nil
	})

	// Start the update loop that refreshes the routing table
	// when HTTPScaledObjects are added, updated, or removed
	eg.Go(func() error {
		setupLog.Info("starting the routing table")

		if err := routingTable.Start(ctx); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "routing table failed")
			return err
		}

		return nil
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	eg.Go(func() error {
		setupLog.Info("starting the admin server", "port", adminPort)

		if err := runAdminServer(ctx, ctrl.Log, adminPort, queues, routingTable); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "admin server failed")
			return err
		}

		return nil
	})

	if metricsCfg.OtelPrometheusExporterEnabled {
		// start the prometheus compatible metrics server
		// serves a prometheus compatible metrics endpoint on the configured port
		eg.Go(func() error {
			if err := runMetricsServer(ctx, ctrl.Log, metricsCfg); !util.IsIgnoredErr(err) {
				setupLog.Error(err, "could not start the Prometheus metrics server")
				return err
			}

			return nil
		})
	}

	// start the proxy servers. This is the server that
	// accepts, holds and forwards user requests
	// start a proxy server with TLS
	if proxyTLSEnabled {
		eg.Go(func() error {
			tlsCfg, err := BuildTLSConfig(TLSOptions{
				CertificatePath:    servingCfg.TLSCertPath,
				KeyPath:            servingCfg.TLSKeyPath,
				CertStorePaths:     servingCfg.TLSCertStorePaths,
				InsecureSkipVerify: servingCfg.TLSSkipVerify,
			}, setupLog)
			if err != nil {
				setupLog.Error(err, "failed to configure TLS")
				return fmt.Errorf("failed to configure TLS: %w", err)
			}

			proxyTLSPort := servingCfg.TLSPort

			setupLog.Info("starting the proxy server with TLS enabled", "port", proxyTLSPort)

			if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, ctrlCache, timeoutCfg, servingCfg, proxyTLSPort, tlsCfg, tracingCfg); !util.IsIgnoredErr(err) {
				setupLog.Error(err, "tls proxy server failed")
				return err
			}
			return nil
		})
	}

	// start a proxy server without TLS.
	eg.Go(func() error {
		setupLog.Info("starting the proxy server with TLS disabled", "port", proxyPort)

		if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, ctrlCache, timeoutCfg, servingCfg, proxyPort, nil, tracingCfg); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "proxy server failed")
			return err
		}

		return nil
	})

	if len(profilingAddr) > 0 {
		eg.Go(func() error {
			setupLog.Info("enabling pprof for profiling", "address", profilingAddr)
			srv := &http.Server{
				Addr:              profilingAddr,
				ReadHeaderTimeout: time.Minute, // mitigate Slowloris attacks
			}
			return srv.ListenAndServe()
		})
	}

	build.PrintComponentInfo(ctrl.Log, "Interceptor")

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		setupLog.Error(err, "fatal error")
		runtime.Goexit()
	}

	setupLog.Info("Bye!")
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	q queue.Counter,
	routingTable routing.Table,
) error {
	lggr = lggr.WithName("runAdminServer")

	probeHandler := handler.NewProbe(routingTable)
	go probeHandler.Start(ctx)

	adminHandler := BuildAdminHandler(lggr, q, probeHandler)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, adminHandler, nil)
}

func runMetricsServer(
	ctx context.Context,
	lggr logr.Logger,
	metricsCfg config.Metrics,
) error {
	lggr.Info("starting the prometheus metrics server", "port", metricsCfg.OtelPrometheusExporterPort, "path", "/metrics")
	addr := fmt.Sprintf("0.0.0.0:%d", metricsCfg.OtelPrometheusExporterPort)
	return kedahttp.ServeContext(ctx, addr, promhttp.Handler(), nil)
}

func runProxyServer(
	ctx context.Context,
	logger logr.Logger,
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable routing.Table,
	reader client.Reader,
	timeouts config.Timeouts,
	serving config.Serving,
	port int,
	tlsCfg *tls.Config,
	tracingConfig config.Tracing,
) error {
	// Build handler chain using the shared builder
	rootHandler := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       logger,
		Queue:        q,
		WaitFunc:     waitFunc,
		RoutingTable: routingTable,
		Reader:       reader,
		Timeouts:     timeouts,
		Serving:      serving,
		TLSConfig:    tlsCfg,
		Tracing:      tracingConfig,
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info("proxy server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, rootHandler, tlsCfg)
}
