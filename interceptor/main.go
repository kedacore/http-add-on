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
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var setupLog = ctrl.Log.WithName("setup")

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes,verbs=get;list;watch
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

	setupLog.Info(
		"starting interceptor",
		"timeoutConfig",
		timeoutCfg,
		"servingConfig",
		servingCfg,
		"metricsConfig",
		metricsCfg,
	)

	proxyTLSEnabled := servingCfg.ProxyTLSEnabled
	profilingAddr := servingCfg.ProfilingAddr

	// setup the configured metrics collectors
	metrics.NewMetricsCollectors(metricsCfg)

	cfg := ctrl.GetConfigOrDie()

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, ctrl.Log)

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

	readyCache, err := k8s.NewReadyEndpointsCacheWithInformer(ctx, ctrl.Log, ctrlCache)
	if err != nil {
		setupLog.Error(err, "creating endpoints cache")
		runtime.Goexit()
	}

	waitFunc := newWorkloadReplicasForwardWaitFunc(readyCache)

	queues := queue.NewMemory()
	routingTable := routing.NewTable(ctrlCache, queues)

	// Setup informers to signal routing table on IR and HTTPSO changes
	routeSources := []client.Object{&v1beta1.InterceptorRoute{}, &v1alpha1.HTTPScaledObject{}}

	for _, obj := range routeSources {
		informer, err := ctrlCache.GetInformer(ctx, obj)
		if err != nil {
			setupLog.Error(err, "getting informer", "type", fmt.Sprintf("%T", obj))
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
	}

	setupLog.Info("Interceptor starting")

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

	// Start the update loop that refreshes the routing table on InterceptorRoute & HTTPSO changes
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
		setupLog.Info("starting the admin server", "port", servingCfg.AdminPort)

		if err := runAdminServer(ctx, ctrl.Log, servingCfg.AdminPort, queues, routingTable); !util.IsIgnoredErr(err) {
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
				MinTLSVersion:      servingCfg.TLSMinVersion,
				MaxTLSVersion:      servingCfg.TLSMaxVersion,
				CipherSuites:       servingCfg.TLSCipherSuites,
				CurvePreferences:   servingCfg.TLSCurvePreferences,
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
		setupLog.Info("starting the proxy server with TLS disabled", "port", servingCfg.ProxyPort)

		if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, ctrlCache, timeoutCfg, servingCfg, servingCfg.ProxyPort, nil, tracingCfg); !util.IsIgnoredErr(err) {
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
