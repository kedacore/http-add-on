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
	echo "github.com/labstack/echo/v4"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	timeoutCfg := config.MustParseTimeouts()
	originCfg := config.MustParseOrigin()
	servingCfg := config.MustParseServing()
	ctx := context.Background()

	deployName := originCfg.TargetDeploymentName
	ns := originCfg.Namespace
	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort
	svcURL, err := originCfg.ServiceURL()
	if err != nil {
		log.Fatalf("Invalid origin service URL: %s", err)
	}

	q := http.NewMemoryQueue()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Kubernetes client config not found (%s)", err)
	}
	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error creating new Kubernetes ClientSet (%s)", err)
	}
	deployInterface := cl.AppsV1().Deployments(ns)
	deployCache, err := k8s.NewK8sDeploymentCache(
		ctx,
		deployInterface,
	)
	if err != nil {
		log.Fatalf("Error creating new deployment cache (%s)", err)
	}
	waitFunc := newDeployReplicasForwardWaitFunc(deployCache, deployName, 1*time.Second)

	go runAdminServer(q, adminPort)

	go runProxyServer(
		q,
		deployName,
		waitFunc,
		svcURL,
		timeoutCfg,
		proxyPort,
	)

	select {}
}

func runAdminServer(q http.QueueCountReader, port int) {
	adminServer := echo.New()
	adminServer.GET("/queue", newQueueSizeHandler(q))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("admin server running on %s", addr)
	log.Fatal(adminServer.Start(addr))
}

func runProxyServer(
	q http.QueueCounter,
	targetDeployName string,
	waitFunc forwardWaitFunc,
	svcURL *url.URL,
	timeouts *config.Timeouts,
	port int,
) {
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())
	proxyHdl := newForwardingHandler(
		svcURL,
		dialContextFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("proxy server starting on %s", addr)
	nethttp.ListenAndServe(addr, countMiddleware(q, proxyHdl))
}
