// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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
//
type queuePinger struct {
	getEndpointsFn k8s.GetEndpointsFunc
	ns             string
	svcName        string
	adminPort      string
	pingMut        *sync.RWMutex
	lastPingTime   time.Time
	allCounts      map[string]int
	aggregateCount int
	lggr           logr.Logger
}

func newQueuePinger(
	ctx context.Context,
	lggr logr.Logger,
	getEndpointsFn k8s.GetEndpointsFunc,
	ns,
	svcName,
	adminPort string,
) (*queuePinger, error) {
	pingMut := new(sync.RWMutex)
	pinger := &queuePinger{
		getEndpointsFn: getEndpointsFn,
		ns:             ns,
		svcName:        svcName,
		adminPort:      adminPort,
		pingMut:        pingMut,
		lggr:           lggr,
		allCounts:      map[string]int{},
		aggregateCount: 0,
	}
	return pinger, pinger.fetchAndSaveCounts(ctx)
}

// start starts the queuePinger
func (q *queuePinger) start(
	ctx context.Context,
	ticker *time.Ticker,
) error {
	lggr := q.lggr.WithName("scaler.queuePinger.start")
	defer ticker.Stop()
	for range ticker.C {
		select {
		case <-ctx.Done():
			lggr.Error(
				ctx.Err(),
				"context marked done. stopping queuePinger loop",
			)
			return errors.Wrap(
				ctx.Err(),
				"context marked done. stopping queuePinger loop",
			)
		default:
			err := q.fetchAndSaveCounts(ctx)
			if err != nil {
				lggr.Error(err, "getting request counts")
				return errors.Wrap(
					err,
					"error getting request counts",
				)
			}
		}
	}
	return nil
}

func (q *queuePinger) counts() map[string]int {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.allCounts
}

func (q *queuePinger) aggregate() int {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.aggregateCount
}

// fetchAndSaveCounts calls fetchCounts, and then
// saves them to internal state in q
func (q *queuePinger) fetchAndSaveCounts(ctx context.Context) error {
	q.pingMut.Lock()
	defer q.pingMut.Unlock()
	counts, agg, err := fetchCounts(
		ctx,
		q.lggr,
		q.getEndpointsFn,
		q.ns,
		q.svcName,
		q.adminPort,
	)
	if err != nil {
		q.lggr.Error(err, "getting request counts")
		return err
	}
	q.allCounts = counts
	q.aggregateCount = agg
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
) (map[string]int, int, error) {
	lggr = lggr.WithName("queuePinger.requestCounts")

	endpointURLs, err := k8s.EndpointsForService(
		ctx,
		ns,
		svcName,
		adminPort,
		endpointsFn,
	)
	if err != nil {
		return nil, 0, err
	}

	countsCh := make(chan *queue.Counts)
	var wg sync.WaitGroup
	fetchGrp, ctx := errgroup.WithContext(ctx)
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
		return nil, 0, err
	}

	// consume the results of the counts channel
	agg := 0
	totalCounts := make(map[string]int)
	// range through the result of each endpoint
	for count := range countsCh {
		// each endpoint returns a map of counts, one count
		// per host. add up the counts for each host
		for host, val := range count.Counts {
			agg += val
			totalCounts[host] += val
		}
	}

	return totalCounts, agg, nil

}
