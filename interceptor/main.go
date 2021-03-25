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
	"github.com/kedacore/http-add-on/pkg/env"
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

// getSvcURL formats the app service name and port into a URL
func getSvcURL() (*url.URL, error) {
	// the name of the service that fronts the user's app
	svcName, err := env.Get("KEDA_HTTP_APP_SERVICE_NAME")
	if err != nil {
		return nil, err
	}
	svcPort, err := env.Get("KEDA_HTTP_APP_SERVICE_PORT")
	if err != nil {
		return nil, err
	}
	hostPortStr := fmt.Sprintf("http://%s:%s", svcName, svcPort)
	return url.Parse(hostPortStr)
}

func main() {
	ctx := context.Background()
	svcURL, err := getSvcURL()
	if err != nil {
		log.Fatalf("Service name / port invalid (%s)", err)
	}
	deployName, err := env.Get("KEDA_HTTP_TARGET_DEPLOYMENT_NAME")
	if err != nil {
		log.Fatal(err)
	}
	ns, err := env.Get("KEDA_HTTP_NAMESPACE")
	if err != nil {
		log.Fatal(err)
	}

	proxyPort, err := env.Get("KEDA_HTTP_PROXY_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_PROXY_PORT missing")
	}
	adminPort, err := env.Get("KEDA_HTTP_ADMIN_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_ADMIN_PORT missing")
	}
	timeouts, err := config.ParseTimeouts()
	if err != nil {
		log.Fatal(err)
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
		timeouts,
		proxyPort,
	)

	select {}
}

func runAdminServer(q http.QueueCountReader, port string) {
	adminServer := echo.New()
	adminServer.GET("/queue", newQueueSizeHandler(q))

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("admin server running on %s", addr)
	log.Fatal(adminServer.Start(addr))
}

func runProxyServer(
	q http.QueueCounter,
	targetDeployName string,
	waitFunc forwardWaitFunc,
	svcURL *url.URL,
	timeouts *config.Timeouts,
	port string,
) {
	proxyMux := nethttp.NewServeMux()
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())
	proxyHdl := newForwardingHandler(
		svcURL,
		dialContextFunc,
		waitFunc,
		timeouts.ResponseHeader,
	)
	proxyMux.Handle("/*", countMiddleware(q, proxyHdl))

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("proxy server starting on %s", addr)
	nethttp.ListenAndServe(addr, proxyMux)
}
