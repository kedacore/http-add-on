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

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(countMiddleware(q))

	e.Any("/*", newForwardingHandler(svcURL))

	adminE := echo.New()
	adminE.Use(middleware.Logger())

	port := fmt.Sprintf(":%s", envOr("PROXY_PORT", "8080"))
	log.Printf("proxy listening on port %s", port)
	log.Fatal(e.Start(port))
}
