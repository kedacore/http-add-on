package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	nethttp "net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
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
	ctx, ctxDone := context.WithCancel(
		context.Background(),
	)

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
		defer ctxDone()
		err := deployCache.StartWatcher(
			ctx,
			lggr,
			time.Duration(servingCfg.DeploymentCachePollIntervalMS)*time.Millisecond,
		)
		lggr.Error(err, "deployment cache watcher failed")
		return err
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
	errGrp.Go(func() error {
		defer ctxDone()
		err := routing.StartConfigMapRoutingTableUpdater(
			ctx,
			lggr,
			time.Duration(servingCfg.RoutingTableUpdateDurationMS)*time.Millisecond,
			configMapsInterface,
			routingTable,
			q,
		)
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
			configMapsInterface,
			q,
			routingTable,
			deployCache,
			adminPort,
			servingCfg,
			timeoutCfg,
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

	// errGrp.Wait() should hang forever for healthy admin and proxy servers.
	// if it returns an error, log and exit immediately.
	waitErr := errGrp.Wait()
	lggr.Error(waitErr, "error with interceptor")
	os.Exit(1)
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	cmGetter k8s.ConfigMapGetter,
	q queue.Counter,
	routingTable *routing.Table,
	deployCache k8s.DeploymentCache,
	port int,
	servingConfig *config.Serving,
	timeoutConfig *config.Timeouts,
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
	adminServer.HandleFunc(
		"/deployments",
		func(w nethttp.ResponseWriter, r *nethttp.Request) {
			if err := json.NewEncoder(w).Encode(deployCache); err != nil {
				lggr.Error(err, "encoding deployment cache")
			}
		},
	)
	kedahttp.AddConfigEndpoint(lggr, adminServer, servingConfig, timeoutConfig)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, adminServer)
}

func runProxyServer(
	ctx context.Context,
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
