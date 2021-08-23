package main

import (
	"context"
	"fmt"
	"math/rand"
	nethttp "net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	lggr, err := pkglog.NewZapr()
	if err != nil {
		fmt.Println("Error building logger", err)
		os.Exit(1)
	}
	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()
	ctx := context.Background()

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort

	cfg, err := rest.InClusterConfig()
	if err != nil {
		lggr.Error(err, "Kubernetes client config not found")
		os.Exit(1)
	}
	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		lggr.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	deployInterface := cl.AppsV1().Deployments(
		servingCfg.CurrentNamespace,
	)
	deployCache, err := k8s.NewK8sDeploymentCache(
		ctx,
		lggr,
		deployInterface,
		time.Duration(servingCfg.DeploymentCachePollIntervalMS)*time.Millisecond,
	)
	if err != nil {
		lggr.Error(err, "creating new deployment cache")
		os.Exit(1)
	}

	configMapsInterface := cl.CoreV1().ConfigMaps(servingCfg.CurrentNamespace)

	waitFunc := newDeployReplicasForwardWaitFunc(deployCache)

	lggr.Info("Interceptor starting")

	q := queue.NewMemory()
	routingTable := routing.NewTable()

	lggr.Info(
		"Fetching initial routing table",
	)
	if err := routing.GetTable(
		ctx,
		lggr,
		configMapsInterface,
		routingTable,
		q,
	); err != nil {
		lggr.Error(err, "fetching routing table")
		os.Exit(1)
	}

	errGrp, ctx := errgroup.WithContext(ctx)

	// start the deployment cache updater
	errGrp.Go(func() error {
		return deployCache.StartWatcher(ctx, lggr)
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
	errGrp.Go(func() error {
		return routing.StartConfigMapRoutingTableUpdater(
			ctx,
			lggr,
			time.Duration(servingCfg.RoutingTableUpdateDurationMS)*time.Millisecond,
			configMapsInterface,
			routingTable,
			q,
		)
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	errGrp.Go(func() error {
		lggr.Info(
			"starting the admin server",
			"port",
			adminPort,
		)
		return runAdminServer(
			lggr,
			configMapsInterface,
			q,
			routingTable,
			adminPort,
		)
	})

	// start the proxy server. this is the server that
	// accepts, holds and forwards user requests
	errGrp.Go(func() error {
		lggr.Info(
			"starting the proxy server",
			"port",
			proxyPort,
		)
		return runProxyServer(
			lggr,
			q,
			waitFunc,
			routingTable,
			timeoutCfg,
			proxyPort,
		)
	})

	// errGrp.Wait() should hang forever for healthy admin and proxy servers.
	// if it returns an error, log and exit immediately.
	waitErr := errGrp.Wait()
	lggr.Error(waitErr, "error with interceptor")
	os.Exit(1)
}

func runAdminServer(
	lggr logr.Logger,
	cmGetter k8s.ConfigMapGetter,
	q queue.Counter,
	routingTable *routing.Table,
	port int,
) error {
	lggr = lggr.WithName("runAdminServer")
	adminServer := nethttp.NewServeMux()
	queue.AddCountsRoute(
		lggr,
		adminServer,
		q,
	)
	routing.AddFetchRoute(
		lggr,
		adminServer,
		routingTable,
	)
	routing.AddPingRoute(
		lggr,
		adminServer,
		cmGetter,
		routingTable,
		q,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return nethttp.ListenAndServe(addr, adminServer)
}

func runProxyServer(
	lggr logr.Logger,
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable *routing.Table,
	timeouts *config.Timeouts,
	port int,
) error {
	lggr = lggr.WithName("runProxyServer")
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())
	proxyHdl := newForwardingHandler(
		lggr,
		routingTable,
		dialContextFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("proxy server starting", "address", addr)
	return nethttp.ListenAndServe(addr, countMiddleware(lggr, q, proxyHdl))
}
