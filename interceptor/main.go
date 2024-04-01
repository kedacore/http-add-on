package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	"github.com/kedacore/http-add-on/pkg/build"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

func main() {

	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if err := config.Validate(servingCfg, *timeoutCfg, ctrl.Log); err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	setupLog.Info(
		"starting interceptor",
		"timeoutConfig",
		timeoutCfg,
		"servingConfig",
		servingCfg,
	)

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort

	cfg := ctrl.GetConfigOrDie()

	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	endpointsCache := k8s.NewInformerBackedEndpointsCache(
		ctrl.Log,
		cl,
		time.Millisecond*time.Duration(servingCfg.EndpointsCachePollIntervalMS),
	)
	if err != nil {
		setupLog.Error(err, "creating new endpoints cache")
		os.Exit(1)
	}
	waitFunc := newWorkloadReplicasForwardWaitFunc(ctrl.Log, endpointsCache)

	httpCl, err := clientset.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "creating new HTTP ClientSet")
		os.Exit(1)
	}

	queues := queue.NewMemory()

	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, servingCfg.ConfigMapCacheRsyncPeriod)
	routingTable, err := routing.NewTable(sharedInformerFactory, servingCfg.WatchNamespace, queues)
	if err != nil {
		setupLog.Error(err, "fetching routing table")
		os.Exit(1)
	}

	setupLog.Info("Interceptor starting")

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, ctrl.Log)

	eg, ctx := errgroup.WithContext(ctx)

	// start the endpoints cache updater
	eg.Go(func() error {
		setupLog.Info("starting the endpoints cache")

		endpointsCache.Start(ctx)
		return nil
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
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

		if err := runAdminServer(ctx, ctrl.Log, queues, adminPort); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "admin server failed")
			return err
		}

		return nil
	})

	// start the proxy server. this is the server that
	// accepts, holds and forwards user requests
	eg.Go(func() error {
		setupLog.Info("starting the proxy server", "port", proxyPort)

		if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, timeoutCfg, proxyPort); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "proxy server failed")
			return err
		}

		return nil
	})

	build.PrintComponentInfo(ctrl.Log, "Interceptor")

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		setupLog.Error(err, "fatal error")
		os.Exit(1)
	}

	setupLog.Info("Bye!")
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	q queue.Counter,
	port int,
) error {
	lggr = lggr.WithName("runAdminServer")
	adminServer := http.NewServeMux()
	queue.AddCountsRoute(
		lggr,
		adminServer,
		q,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, adminServer)
}

func runProxyServer(
	ctx context.Context,
	logger logr.Logger,
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable routing.Table,
	timeouts *config.Timeouts,
	port int,
) error {
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())

	probeHandler := handler.NewProbe([]util.HealthChecker{
		routingTable,
	})
	go probeHandler.Start(ctx)

	var upstreamHandler http.Handler
	upstreamHandler = newForwardingHandler(
		logger,
		dialContextFunc,
		waitFunc,
		newForwardingConfigFromTimeouts(timeouts),
	)
	upstreamHandler = middleware.NewCountingMiddleware(
		q,
		upstreamHandler,
	)

	var rootHandler http.Handler
	rootHandler = middleware.NewRouting(
		routingTable,
		probeHandler,
		upstreamHandler,
	)
	rootHandler = middleware.NewLogging(
		logger,
		rootHandler,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info("proxy server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, rootHandler)
}
