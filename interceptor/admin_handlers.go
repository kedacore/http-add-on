package main

import (
	"log"

	"github.com/kedacore/http-add-on/pkg/http"
	echo "github.com/labstack/echo/v4"
)

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func newQueueSizeHandler(q http.QueueCountReader) echo.HandlerFunc {
	return func(c echo.Context) error {
		cur, err := q.Current()
		if err != nil {
			log.Printf("Error getting queue size (%s)", err)
			c.Error(err)
			return err
		}
		return c.JSON(200, cur)
	}
}
