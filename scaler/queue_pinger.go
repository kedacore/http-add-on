// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
)

type queuePinger struct {
	getEndpointsFn k8s.GetEndpointsFunc
	ns             string
	svcName        string
	adminPort      string
	pingMut        *sync.RWMutex
	lastPingTime   time.Time
	allCounts      map[string]int
	lggr           logr.Logger
}

func newQueuePinger(
	ctx context.Context,
	lggr logr.Logger,
	getEndpointsFn k8s.GetEndpointsFunc,
	ns,
	svcName,
	adminPort string,
	pingTicker *time.Ticker,
) *queuePinger {
	pingMut := new(sync.RWMutex)
	pinger := &queuePinger{
		getEndpointsFn: getEndpointsFn,
		ns:             ns,
		svcName:        svcName,
		adminPort:      adminPort,
		pingMut:        pingMut,
		lggr:           lggr,
	}

	go func() {
		defer pingTicker.Stop()
		for range pingTicker.C {
			if err := pinger.requestCounts(ctx); err != nil {
				lggr.Error(err, "getting request counts")
			}
		}
	}()

	return pinger
}

func (q *queuePinger) counts() map[string]int {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.allCounts
}

func (q *queuePinger) requestCounts(ctx context.Context) error {
	lggr := q.lggr.WithName("queuePinger.requestCounts")

	endpointURLs, err := k8s.EndpointsForService(
		ctx,
		q.ns,
		q.svcName,
		q.adminPort,
		q.getEndpointsFn,
	)
	if err != nil {
		return err
	}

	countsCh := make(chan *queue.Counts)
	var wg sync.WaitGroup
	for _, endpoint := range endpointURLs {
		wg.Add(1)
		go func(u *url.URL) {
			defer wg.Done()

			counts, err := queue.GetCounts(
				ctx,
				lggr,
				http.DefaultClient,
				*u,
			)
			if err != nil {
				lggr.Error(
					err,
					"getting queue counts from interceptor",
					"interceptorAddress",
					u.String(),
				)
				return
			}
			countsCh <- counts
		}(endpoint)
	}

	go func() {
		wg.Wait()
		close(countsCh)
	}()

	totalCounts := make(map[string]int)
	for count := range countsCh {
		for host, val := range count.Counts {
			totalCounts[host] += val
		}
	}

	q.pingMut.Lock()
	defer q.pingMut.Unlock()
	q.allCounts = totalCounts
	q.lastPingTime = time.Now()

	return nil

}
