package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	nethttp "net/http"
	"net/url"
	"time"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	echo "github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	timeoutCfg := config.MustParseTimeouts()
	operatorCfg := config.MustParseOperator()
	servingCfg := config.MustParseServing()
	ctx := context.Background()

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort

	q := http.NewMemoryQueue()

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
	routingTable, err := fetchRoutingTable(ctx, operatorRoutingFetchURL)
	if err != nil {
		log.Fatal("error fetching routing table ", err)
	}

	errGrp, ctx := errgroup.WithContext(ctx)
	errGrp.Go(func() error {
		return runAdminServer(
			operatorRoutingFetchURL,
			q,
			routingTable,
			adminPort,
		)
	})
	errGrp.Go(func() error {
		log.Printf(
			"routing table update loop starting for operator URL %v",
			*operatorRoutingFetchURL,
		)
		return routing.StartUpdateLoop(
			ctx,
			operatorCfg.RoutingTableUpdateDuration(),
			operatorRoutingFetchURL,
			routingTable,
			func(ctx context.Context) (*routing.Table, error) {
				return fetchRoutingTable(ctx, operatorRoutingFetchURL)
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
	routingFetchURL *url.URL,
	q http.QueueCountReader,
	routingTable *routing.Table,
	port int,
) error {
	adminServer := echo.New()
	adminServer.GET(
		"/queue",
		newQueueSizeHandler(q),
	)
	adminServer.GET(
		"/routing_ping",
		newRoutingPingHandler(routingFetchURL, routingTable),
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("admin server running on %s", addr)
	return adminServer.Start(addr)
}

func runProxyServer(
	q http.QueueCounter,
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
