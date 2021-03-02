package main

import (
	"math"
	"sync"
	"time"

	echo "github.com/labstack/echo/v4"
)

func (i *InterceptorSuite) TestCountMiddleware() {
	queueCounter := &fakeQueueCounter{}
	middleware := countMiddleware(queueCounter)
	var wg sync.WaitGroup
	wg.Add(1)
	handler := middleware(func(c echo.Context) error {
		wg.Done()
		return c.String(200, "OK")
	})
	_, echoCtx, _ := newTestCtx("GET", "/something")
	go func() {
		i.NoError(handler(echoCtx))
	}()

	// after the handler was called, first wait for it to complete.
	// then check to make sure the pending queue size was increased and decreased.
	//
	// the increase and decrease operations happen in goroutines, so the ordering
	// isn't guaranteed
	wg.Wait()
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	resizes := []int{}
	done := false
	for i := 0; i < 2; i++ {
		if len(resizes) == 2 || done {
			break
		}
		select {
		case i := <-queueCounter.resizedCh:
			resizes = append(resizes, i)
		case <-timer.C:
			done = true
			break
		}
	}
	agg := 0
	for _, delta := range resizes {
		i.Equal(1, math.Abs(float64(delta)))
		agg += delta
	}
	i.Equal(0, agg, "sum of all the resize operations")
}
