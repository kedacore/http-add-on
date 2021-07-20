package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/require"
)

func TestCountMiddleware(t *testing.T) {
	r := require.New(t)
	queueCounter := queue.NewFakeCounter()
	middleware := countMiddleware(
		queueCounter,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	const timeout = 5000 * time.Millisecond
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// we expect the queue to be resized twice. once to mark
	// a pending HTTP request, then a second time to remove it.
	// by the end of both sends, resize1 + resize2 should be 0,
	// or in other words, the queue size should be back to zero
	agg := 0
	for i := 0; i < 2; i++ {
		select {
		case delta := <-queueCounter.ResizedCh:
			r.Equal(float64(1), math.Abs(float64(delta)))
			agg += delta
		case <-timer.C:
			r.FailNow(
				"timed out waiting for the count middleware",
				"timeout was %s, iteration %d",
				timeout,
				i,
			)
		}
	}

	r.Equal(0, agg, "sum of all the resize operations")
	r.Equal(200, rec.Code)
	r.Equal("OK", rec.Body.String())
}
