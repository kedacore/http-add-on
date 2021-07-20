package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	nethttp "net/http"
	"net/url"
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
		log.Fatalf("Error building logger (%v)", err)
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
		log.Fatalf("Kubernetes client config not found (%s)", err)
	}
	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error creating new Kubernetes ClientSet (%s)", err)
	}
	deployInterface := cl.AppsV1().Deployments(
		servingCfg.CurrentNamespace,
	)
	deployCache, err := k8s.NewK8sDeploymentCache(
		ctx,
		deployInterface,
	)
	if err != nil {
		log.Fatalf("Error creating new deployment cache (%s)", err)
	}
	waitFunc := newDeployReplicasForwardWaitFunc(deployCache, 1*time.Second)

	operatorRoutingFetchURL, err := operatorCfg.RoutingFetchURL()
	if err != nil {
		log.Fatal("error getting the operator URL ", err)
	}

	log.Printf("Interceptor starting")
	log.Printf("Fetching initial routing table")
	routingTable, err := fetchRoutingTable(ctx, lggr, operatorRoutingFetchURL)
	if err != nil {
		log.Fatal("error fetching routing table ", err)
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
		log.Printf(
			"routing table update loop starting for operator URL %v",
			operatorRoutingFetchURL.String(),
		)
		return routing.StartUpdateLoop(
			ctx,
			operatorCfg.RoutingTableUpdateDuration(),
			routingTable,
			func(ctx context.Context) (*routing.Table, error) {
				return fetchRoutingTable(ctx, lggr, operatorRoutingFetchURL)
			},
		)
	})

	errGrp.Go(func() error {
		return runProxyServer(
			q,
			waitFunc,
			routingTable,
			timeoutCfg,
			proxyPort,
		)
	})

	log.Fatal(errGrp.Wait())
}

func runAdminServer(
	lggr logr.Logger,
	routingFetchURL *url.URL,
	q queue.CountReader,
	routingTable *routing.Table,
	port int,
) error {
	adminServer := nethttp.NewServeMux()
	adminServer.Handle(
		queue.CountsPath,
		queue.NewSizeHandler(lggr, q),
	)
	adminServer.Handle(
		"/routing_ping",
		newRoutingPingHandler(lggr, routingFetchURL, routingTable),
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("admin server running on %s", addr)
	return nethttp.ListenAndServe(addr, adminServer)
}

func runProxyServer(
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable *routing.Table,
	timeouts *config.Timeouts,
	port int,
) error {
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
	log.Printf("proxy server starting on %s", addr)
	return nethttp.ListenAndServe(addr, countMiddleware(q, proxyHdl))
}
