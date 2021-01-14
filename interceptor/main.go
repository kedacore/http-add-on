package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"time"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/kedacore/http-add-on/pkg/http"
	echo "github.com/labstack/echo/v4"
	middleware "github.com/labstack/echo/v4/middleware"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// getSvcURL formats the app service name and port into a URL
func getSvcURL() (*url.URL, error) {
	svcName, err := env.Get("KEDA_HTTP_SVC_NAME")
	if err != nil {
		return nil, err
	}
	svcPort, err := env.Get("KEDA_HTTP_SVC_PORT")
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

	q := http.NewMemoryQueue()
	proxyServer := echo.New()
	adminServer := echo.New()

	proxyServer.Use(middleware.Logger())
	proxyServer.Use(countMiddleware(q)) // adds the request counting middleware

	// forwards any request to the destination app after counting
	proxyServer.Any("/*", newForwardingHandler(svcURL))

	adminServer.GET("/queue", newQueueSizeHandler(q))

	proxyPort := env.GetOr("KEDA_HTTP_PROXY_PORT", "8080")
	adminPort := env.GetOr("KEDA_HTTP_ADMIN_PORT", "8081")
	go runServer("proxy", proxyServer, proxyPort)
	go runServer("admin", adminServer, adminPort)

	select {}
}

func runServer(name string, e *echo.Echo, port string) {
	log.Printf("%s server running on port %s", name, port)
	log.Fatal(e.Start(port))
}
