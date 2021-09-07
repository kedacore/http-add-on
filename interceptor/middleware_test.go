package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestCountMiddleware(t *testing.T) {
	ctx := context.Background()
	const host = "testingkeda.com"
	r := require.New(t)
	queueCounter := queue.NewFakeCounter()
	middleware := countMiddleware(
		logr.Discard(),
		queueCounter,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("OK"))
		}),
	)

	// no host in the request
	req, err := http.NewRequest("GET", "/something", nil)
	r.NoError(err)
	agg, respRecorder := expectResizes(
		ctx,
		t,
		0,
		middleware,
		req,
		queueCounter,
		func(t *testing.T, hostAndCount queue.HostAndCount) {},
	)
	r.Equal(400, respRecorder.Code)
	r.Equal("Host not found, not forwarding request", respRecorder.Body.String())
	r.Equal(0, agg)

	// run middleware with the host in the request
	req, err = http.NewRequest("GET", "/something", nil)
	r.NoError(err)
	req.Host = host
	// for a valid request, we expect the queue to be resized twice.
	// once to mark a pending HTTP request, then a second time to remove it.
	// by the end of both sends, resize1 + resize2 should be 0,
	// or in other words, the queue size should be back to zero
	agg, respRecorder = expectResizes(
		ctx,
		t,
		2,
		middleware,
		req,
		queueCounter,
		func(t *testing.T, hostAndCount queue.HostAndCount) {
			t.Helper()
			r := require.New(t)
			r.Equal(float64(1), math.Abs(float64(hostAndCount.Count)))
			r.Equal(host, hostAndCount.Host)
		},
	)
	r.Equal(200, respRecorder.Code)
	r.Equal("OK", respRecorder.Body.String())
	r.Equal(0, agg)
}

// expectResizes creates a new httptest.ResponseRecorder, then passes req through
// the middleware. every time the middleware calls fakeCounter.Resize(), it calls
// resizeCheckFn with t and the queue.HostCount that represents the resize call
// that was made. it also maintains an aggregate delta of the counts passed to
// Resize. If, for example, the following integers were passed to resize over
// 4 calls: [-1, 1, 1, 2], the aggregate would be -1+1+1+2=3
//
// this function returns the aggregate and the httptest.ResponseRecorder that was
// created and used with the middleware
func expectResizes(
	ctx context.Context,
	t *testing.T,
	nResizes int,
	middleware http.Handler,
	req *http.Request,
	fakeCounter *queue.FakeCounter,
	resizeCheckFn func(*testing.T, queue.HostAndCount),
) (int, *httptest.ResponseRecorder) {
	t.Helper()
	r := require.New(t)
	const timeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
	agg := 0
	grp.Go(func() error {
		// we expect the queue to be resized nResizes times
		for i := 0; i < nResizes; i++ {
			select {
			case hostAndCount := <-fakeCounter.ResizedCh:
				agg += hostAndCount.Count
				resizeCheckFn(t, hostAndCount)
			case <-ctx.Done():
				return fmt.Errorf(
					"timed out waiting for the count middleware. expected %d resizes, timeout was %s, iteration %d",
					nResizes,
					timeout,
					i,
				)
			}
		}
		return nil
	})

	respRecorder := httptest.NewRecorder()
	middleware.ServeHTTP(respRecorder, req)

	r.NoError(grp.Wait())

	return agg, respRecorder
}
