// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
)

type PingerStatus int32

const (
	PingerUNKNOWN PingerStatus = 0
	PingerACTIVE  PingerStatus = 1
	PingerERROR   PingerStatus = 2
)

// queuePinger has functionality to ping all interceptors
// behind a given `Service`, fetch their pending queue counts,
// and aggregate all of those counts together.
//
// It's capable of doing that work in parallel when possible
// as well.
//
// Sample usage:
//
//	pinger, err := newQueuePinger(ctx, lggr, getEndpointsFn, ns, svcName, adminPort)
//	if err != nil {
//		panic(err)
//	}
//	// make sure to start the background pinger loop.
//	// you can shut this loop down by using a cancellable
//	// context
//	go pinger.start(ctx, ticker)
type queuePinger struct {
	getEndpointsFn     k8s.GetEndpointsFunc
	interceptorNS      string
	interceptorSvcName string
	adminPort          string
	pingMut            *sync.RWMutex
	lastPingTime       time.Time
	allCounts          map[string]queue.Count
	lggr               logr.Logger
	status             PingerStatus
}

func newQueuePinger(
	lggr logr.Logger,
	getEndpointsFn k8s.GetEndpointsFunc,
	ns,
	svcName,
	adminPort string,
) *queuePinger {
	pingMut := new(sync.RWMutex)
	pinger := &queuePinger{
		getEndpointsFn:     getEndpointsFn,
		interceptorNS:      ns,
		interceptorSvcName: svcName,
		adminPort:          adminPort,
		pingMut:            pingMut,
		lggr:               lggr,
		allCounts:          map[string]queue.Count{},
	}
	return pinger
}

// start starts the queuePinger
func (q *queuePinger) start(
	ctx context.Context,
	ticker *time.Ticker,
	endpCache k8s.EndpointsCache,
) error {
	endpoWatchIface, err := endpCache.Watch(q.interceptorNS, q.interceptorSvcName)
	if err != nil {
		return err
	}
	endpEvtChan := endpoWatchIface.ResultChan()
	defer endpoWatchIface.Stop()

	lggr := q.lggr.WithName("scaler.queuePinger.start")
	defer ticker.Stop()
	for {
		select {
		// handle cancellations/timeout
		case <-ctx.Done():
			lggr.Error(
				ctx.Err(),
				"context marked done. stopping queuePinger loop",
			)
			q.status = PingerERROR
			return fmt.Errorf("context marked done. stopping queuePinger loop: %w", ctx.Err())
		// do our regularly scheduled work
		case <-ticker.C:
			err := q.fetchAndSaveCounts(ctx)
			if err != nil {
				lggr.Error(err, "getting request counts")
			}
		// handle changes to the interceptor fleet
		// Endpoints
		case <-endpEvtChan:
			err := q.fetchAndSaveCounts(ctx)
			if err != nil {
				lggr.Error(
					err,
					"getting request counts after interceptor endpoints event",
				)
			}
		}
	}
}

func (q *queuePinger) counts() map[string]queue.Count {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.allCounts
}

// fetchAndSaveCounts calls fetchCounts, and then
// saves them to internal state in q
func (q *queuePinger) fetchAndSaveCounts(ctx context.Context) error {
	q.pingMut.Lock()
	defer q.pingMut.Unlock()
	counts, err := fetchCounts(
		ctx,
		q.lggr,
		q.getEndpointsFn,
		q.interceptorNS,
		q.interceptorSvcName,
		q.adminPort,
	)
	if err != nil {
		q.lggr.Error(err, "getting request counts")
		q.status = PingerERROR
		return err
	}
	q.status = PingerACTIVE
	q.allCounts = counts
	q.lastPingTime = time.Now()

	return nil
}

// fetchCounts fetches all counts from every endpoint returned
// by endpointsFn for the given service named svcName on the
// port adminPort, in namespace ns.
//
// Requests to fetch endpoints are made concurrently and
// aggregated when all requests return successfully.
//
// Upon any failure, a non-nil error is returned and the
// other two return values are nil and 0, respectively.
func fetchCounts(
	ctx context.Context,
	lggr logr.Logger,
	endpointsFn k8s.GetEndpointsFunc,
	ns,
	svcName,
	adminPort string,
) (map[string]queue.Count, error) {
	lggr = lggr.WithName("queuePinger.requestCounts")

	endpointURLs, err := k8s.EndpointsForService(
		ctx,
		ns,
		svcName,
		adminPort,
		endpointsFn,
	)
	if err != nil {
		return nil, err
	}

	if len(endpointURLs) == 0 {
		return nil, fmt.Errorf("there isn't any valid interceptor endpoint")
	}

	countsCh := make(chan *queue.Counts)
	var wg sync.WaitGroup
	fetchGrp, _ := errgroup.WithContext(ctx)
	for _, endpoint := range endpointURLs {
		// capture the endpoint in a loop-local
		// variable so that the goroutine can
		// use it
		u := endpoint
		// have the errgroup goroutine send to
		// a "private" goroutine, which we'll
		// then forward on to countsCh
		ch := make(chan *queue.Counts)
		wg.Add(1)
		fetchGrp.Go(func() error {
			counts, err := queue.GetCounts(
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
				return err
			}
			ch <- counts
			return nil
		})
		// forward the "private" goroutine
		// on to countsCh separately
		go func() {
			defer wg.Done()
			res := <-ch
			countsCh <- res
		}()
	}

	// close countsCh after all goroutines are done sending
	// to their "private" channels, so that we can range
	// over countsCh normally below
	go func() {
		wg.Wait()
		close(countsCh)
	}()

	if err := fetchGrp.Wait(); err != nil {
		lggr.Error(err, "fetching all counts failed")
		return nil, err
	}

	totalCounts := make(map[string]queue.Count)
	// range through the result of each endpoint
	for count := range countsCh {
		// each endpoint returns a map of counts, one count
		// per host. add up the counts for each host
		for host, val := range count.Counts {
			var responseCount queue.Count
			var ok bool
			if responseCount, ok = totalCounts[host]; !ok {
				responseCount = queue.Count{}
			}
			responseCount.Concurrency += val.Concurrency
			responseCount.RPS += val.RPS
			totalCounts[host] = responseCount
		}
	}

	return totalCounts, nil
}
