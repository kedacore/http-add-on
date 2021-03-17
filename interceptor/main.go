package main

import (
	"fmt"
	"log"
	"math/rand"
	nethttp "net/http"
	"net/url"
	"time"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/kedacore/http-add-on/pkg/http"
	echo "github.com/labstack/echo/v4"
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
	svcURL, err := getSvcURL()
	if err != nil {
		log.Fatalf("Service name / port invalid (%s)", err)
	}

	proxyPort, err := env.Get("KEDA_HTTP_PROXY_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_PROXY_PORT missing")
	}
	adminPort, err := env.Get("KEDA_HTTP_ADMIN_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_ADMIN_PORT missing")
	}
	proxyTimeoutMS := env.GetIntOr("KEDA_HTTP_PROXY_TIMEOUT", 1000)

	q := http.NewMemoryQueue()

	go runAdminServer(q, adminPort)
	go runProxyServer(
		q,
		svcURL,
		time.Duration(proxyTimeoutMS)*time.Millisecond,
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
	svcURL *url.URL,
	proxyTimeout time.Duration,
	port string,
) {
	proxyMux := nethttp.NewServeMux()
	proxyHdl := newForwardingHandler(svcURL, proxyTimeout)
	proxyMux.Handle("/*", countMiddleware(q, proxyHdl))

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("proxy server starting on %s", addr)
	nethttp.ListenAndServe(addr, proxyMux)
}
