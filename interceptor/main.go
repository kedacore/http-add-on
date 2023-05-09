package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kedacore/http-add-on/interceptor/config"
	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	"github.com/kedacore/http-add-on/pkg/build"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch

func main() {
	lggr, err := pkglog.NewZapr()
	if err != nil {
		fmt.Println("Error building logger", err)
		os.Exit(1)
	}
	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()
	if err := config.Validate(*servingCfg, *timeoutCfg); err != nil {
		lggr.Error(err, "invalid configuration")
		os.Exit(1)
	}
	ctx, ctxDone := context.WithCancel(
		context.Background(),
	)
	lggr.Info(
		"starting interceptor",
		"timeoutConfig",
		timeoutCfg,
		"servingConfig",
		servingCfg,
	)

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort

	cfg, err := ctrl.GetConfig()
	if err != nil {
		lggr.Error(err, "Kubernetes client config not found")
		os.Exit(1)
	}
	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		lggr.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	deployCache := k8s.NewInformerBackedDeploymentCache(
		lggr,
		cl,
		time.Millisecond*time.Duration(servingCfg.DeploymentCachePollIntervalMS),
	)
	if err != nil {
		lggr.Error(err, "creating new deployment cache")
		os.Exit(1)
	}

	waitFunc := newDeployReplicasForwardWaitFunc(lggr, deployCache)

	lggr.Info("Interceptor starting")

	q := queue.NewMemory()

	httpCl, err := clientset.NewForConfig(cfg)
	if err != nil {
		lggr.Error(err, "creating new HTTP ClientSet")
		os.Exit(1)
	}
	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, servingCfg.ConfigMapCacheRsyncPeriod)
	routingTable, err := routing.NewTable(sharedInformerFactory, servingCfg.WatchNamespace)
	if err != nil {
		lggr.Error(err, "fetching routing table")
		os.Exit(1)
	}

	errGrp, ctx := errgroup.WithContext(ctx)

	// start the deployment cache updater
	errGrp.Go(func() error {
		defer ctxDone()
		err := deployCache.Start(ctx)
		lggr.Error(err, "deployment cache watcher failed")
		return err
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
	errGrp.Go(func() error {
		defer ctxDone()
		err := routingTable.Start(ctx)
		lggr.Error(err, "config map routing table updater failed")
		return err
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	errGrp.Go(func() error {
		defer ctxDone()
		lggr.Info(
			"starting the admin server",
			"port",
			adminPort,
		)
		err := runAdminServer(
			ctx,
			lggr,
			q,
			adminPort,
		)
		lggr.Error(err, "admin server failed")
		return err
	})

	// start the proxy server. this is the server that
	// accepts, holds and forwards user requests
	errGrp.Go(func() error {
		defer ctxDone()
		lggr.Info(
			"starting the proxy server",
			"port",
			proxyPort,
		)
		err := runProxyServer(
			ctx,
			lggr,
			q,
			waitFunc,
			routingTable,
			timeoutCfg,
			proxyPort,
		)
		lggr.Error(err, "proxy server failed")
		return err
	})
	build.PrintComponentInfo(lggr, "Interceptor")

	// errGrp.Wait() should hang forever for healthy admin and proxy servers.
	// if it returns an error, log and exit immediately.
	waitErr := errGrp.Wait()
	lggr.Error(waitErr, "error with interceptor")
	os.Exit(1)
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
	lggr logr.Logger,
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable routing.Table,
	timeouts *config.Timeouts,
	port int,
) error {
	lggr = lggr.WithName("runProxyServer")
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())
	proxyHdl := countMiddleware(
		lggr,
		q,
		newForwardingHandler(
			lggr,
			routingTable,
			dialContextFunc,
			waitFunc,
			newForwardingConfigFromTimeouts(timeouts),
		),
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("proxy server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, proxyHdl)
}
