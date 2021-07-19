package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCountMiddleware(t *testing.T) {
	r := require.New(t)
	queueCounter := &fakeQueueCounter{}
	var wg sync.WaitGroup
	wg.Add(1)
	middleware := countMiddleware(
		queueCounter,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wg.Done()
			w.WriteHeader(200)
			w.Write([]byte("OK"))
		}),
	)
	req, err := http.NewRequest("GET", "/something", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()

	go func() {
		middleware.ServeHTTP(rec, req)
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
			// effectively breaks out of the outer loop.
			// putting a 'break' here will only break out
			// of the select block
			done = true
		}
	}
	agg := 0
	for _, delta := range resizes {
		r.Equal(1, math.Abs(float64(delta)))
		agg += delta
	}
	r.Equal(0, agg, "sum of all the resize operations")
}
