package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"time"

	"github.com/kedacore/http-add-on/pkg/http"
	echo "github.com/labstack/echo/v4"
	middleware "github.com/labstack/echo/v4/middleware"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// getSvcURL formats the app service name and port into a URL
func getSvcURL() (*url.URL, error) {
	svcName, err := env("KEDA_HTTP_SVC_NAME")
	if err != nil {
		return nil, err
	}
	svcPort, err := env("KEDA_HTTP_SVC_PORT")
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

	// TODO: make configurable (obv need to build more)
	q := http.NewMemoryQueue()
	httpServer := echo.New()

	httpServer.Use(middleware.Logger())
	httpServer.Use(countMiddleware(q)) // adds the request counting middleware

	// forwards any request to the destination app after counting
	httpServer.Any("/*", newForwardingHandler(svcURL))

	port := "8080"
	log.Printf("proxy listening on port %s", port)
	log.Fatal(httpServer.Start(port))
}
