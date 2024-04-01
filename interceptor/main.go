package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	"github.com/kedacore/http-add-on/pkg/build"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

func main() {
	lggr, err := pkglog.NewZapr()
	if err != nil {
		fmt.Println("Error building logger", err)
		os.Exit(1)
	}

	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()
	if err := config.Validate(servingCfg, *timeoutCfg, lggr); err != nil {
		lggr.Error(err, "invalid configuration")
		os.Exit(1)
	}

	lggr.Info(
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
		lggr.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	endpointsCache := k8s.NewInformerBackedEndpointsCache(
		lggr,
		cl,
		time.Millisecond*time.Duration(servingCfg.EndpointsCachePollIntervalMS),
	)
	if err != nil {
		lggr.Error(err, "creating new endpoints cache")
		os.Exit(1)
	}
	waitFunc := newWorkloadReplicasForwardWaitFunc(lggr, endpointsCache)

	httpCl, err := clientset.NewForConfig(cfg)
	if err != nil {
		lggr.Error(err, "creating new HTTP ClientSet")
		os.Exit(1)
	}

	queues := queue.NewMemory()

	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, servingCfg.ConfigMapCacheRsyncPeriod)
	routingTable, err := routing.NewTable(sharedInformerFactory, servingCfg.WatchNamespace, queues)
	if err != nil {
		lggr.Error(err, "fetching routing table")
		os.Exit(1)
	}

	lggr.Info("Interceptor starting")

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, lggr)

	eg, ctx := errgroup.WithContext(ctx)

	// start the endpoints cache updater
	eg.Go(func() error {
		lggr.Info("starting the endpoints cache")

		endpointsCache.Start(ctx)
		return nil
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
	eg.Go(func() error {
		lggr.Info("starting the routing table")

		if err := routingTable.Start(ctx); !util.IsIgnoredErr(err) {
			lggr.Error(err, "routing table failed")
			return err
		}

		return nil
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	eg.Go(func() error {
		lggr.Info("starting the admin server", "port", adminPort)

		if err := runAdminServer(ctx, lggr, queues, adminPort); !util.IsIgnoredErr(err) {
			lggr.Error(err, "admin server failed")
			return err
		}

		return nil
	})

	// start the proxy server. this is the server that
	// accepts, holds and forwards user requests
	eg.Go(func() error {
		lggr.Info("starting the proxy server", "port", proxyPort)

		if err := runProxyServer(ctx, lggr, queues, waitFunc, routingTable, timeoutCfg, proxyPort); !util.IsIgnoredErr(err) {
			lggr.Error(err, "proxy server failed")
			return err
		}

		return nil
	})

	build.PrintComponentInfo(lggr, "Interceptor")

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		lggr.Error(err, "fatal error")
		os.Exit(1)
	}

	lggr.Info("Bye!")
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
