package main

import (
	"context"
	"fmt"
	"math/rand"
	nethttp "net/http"
	"os"

	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	echo "github.com/labstack/echo/v4"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	logger := klogr.New()
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
		logger.Error(err, "Invalid origin service URL")
		os.Exit(1)
	}

	q := http.NewMemoryQueue()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		logger.Error(err, "Finding Kubernetes config in cluster")
		os.Exit(1)
	}
	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "creating new Kubernetes client")
		os.Exit(1)
	}
	deployInterface := cl.AppsV1().Deployments(ns)
	deployCache, err := k8s.NewK8sDeploymentCache(
		ctx,
		deployInterface,
	)
	if err != nil {
		logger.Error(err, "creating new deployment cache")
		os.Exit(1)
	}
	waitFunc := newDeployReplicasForwardWaitFunc(
		logger,
		deployCache,
		deployName,
		1*time.Second,
	)

	logger.Info(
		"Interceptor Starting",
		"appServiceName",
		originCfg.AppServiceName,
		"appServicePort",
		originCfg.AppServicePort,
		"targetDeploymentName",
		originCfg.TargetDeploymentName,
	)

	go runAdminServer(logger, q, adminPort)

	go runProxyServer(
		logger,
		q,
		deployName,
		waitFunc,
		svcURL,
		timeoutCfg,
		proxyPort,
	)

	select {}
}

func runAdminServer(
	logger logr.Logger,
	q http.QueueCountReader,
	port int,
) {
	logger = logger.WithName("runAdminServer")
	adminServer := echo.New()
	adminServer.GET("/queue", newQueueSizeHandler(q))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info(
		"starting admin server",
		"address",
		addr,
	)
	logger.Error(adminServer.Start(addr), "running admin server")
}

func runProxyServer(
	logger logr.Logger,
	q http.QueueCounter,
	targetDeployName string,
	waitFunc forwardWaitFunc,
	svcURL *url.URL,
	timeouts *config.Timeouts,
	port int,
) {
	logger = logger.WithName("runProxyServer")
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(
		logger,
		dialer,
		timeouts.DefaultBackoff(),
	)
	proxyHdl := newForwardingHandler(
		logger,
		svcURL,
		dialContextFunc,
		waitFunc,
		timeouts.DeploymentReplicas,
		timeouts.ResponseHeader,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info("proxy server starting", "address", addr)
	logger.Error(
		nethttp.ListenAndServe(addr, countMiddleware(q, proxyHdl)),
		"running proxy server",
	)
}
