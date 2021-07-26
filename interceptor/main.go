package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	nethttp "net/http"
	"net/url"
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
	operatorCfg := config.MustParseOperator()
	servingCfg := config.MustParseServing()
	ctx := context.Background()

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort

	q := queue.NewMemory()

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
		deployInterface,
	)
	if err != nil {
		lggr.Error(err, "creating new deployment cache")
		os.Exit(1)
	}
	waitFunc := newDeployReplicasForwardWaitFunc(deployCache, 1*time.Second)

	operatorRoutingFetchURL, err := operatorCfg.RoutingFetchURL()
	if err != nil {
		lggr.Error(err, "getting the operator URL")
		os.Exit(1)
	}

	lggr.Info("Interceptor starting")
	lggr.Info(
		"Fetching initial routing table",
		"operatorRoutingURL",
		operatorRoutingFetchURL.String(),
	)
	routingTable, err := routing.GetTable(
		ctx,
		nethttp.DefaultClient,
		lggr,
		*operatorRoutingFetchURL,
	)
	if err != nil {
		lggr.Error(err, "fetching routing table")
		os.Exit(1)
	}

	errGrp, ctx := errgroup.WithContext(ctx)
	errGrp.Go(func() error {
		return runAdminServer(
			lggr,
			operatorRoutingFetchURL,
			q,
			routingTable,
			adminPort,
		)
	})
	errGrp.Go(func() error {
		lggr.Info(
			"routing table update loop starting",
			"operatorRoutingURL",
			operatorRoutingFetchURL.String(),
		)
		return routing.StartUpdateLoop(
			ctx,
			operatorCfg.RoutingTableUpdateDuration(),
			routingTable,
			func(ctx context.Context) (*routing.Table, error) {
				return routing.GetTable(
					ctx,
					nethttp.DefaultClient,
					lggr,
					*operatorRoutingFetchURL,
				)
			},
		)
	})

	errGrp.Go(func() error {
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
	routingFetchURL *url.URL,
	q queue.CountReader,
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
	routing.AddPingRoute(
		lggr,
		adminServer,
		routingTable,
	)

	adminServer.HandleFunc(
		"/routing_table",
		func(w nethttp.ResponseWriter, r *nethttp.Request) {
			if err := json.NewEncoder(w).Encode(routingTable); err != nil {
				w.WriteHeader(500)
				lggr.Error(err, "encoding routing table")
			}
		},
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
		routingTable,
		dialContextFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("proxy server starting", "address", addr)
	return nethttp.ListenAndServe(addr, countMiddleware(q, proxyHdl))
}
