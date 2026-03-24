// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
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

	defaultWindow      = time.Minute
	defaultGranularity = time.Second
)

// queuePinger has functionality to ping all interceptors
// behind a given `Service`, fetch their pending queue counts,
// and aggregate all of those counts together.
//
// It computes request rate (RPS) from monotonic counters
// reported by interceptor pods, using a windowed ring buffer.
//
// Sample usage:
//
//	pinger := newQueuePinger(lggr, getEndpointsFn, ns, svcName, deplName, adminPort)
//	go pinger.start(ctx, ticker)
type queuePinger struct {
	getEndpointsFn         k8s.GetEndpointsFunc
	interceptorNS          string
	interceptorSvcName     string
	interceptorServiceName string
	adminPort              string
	pingMut                sync.RWMutex
	lastPingTime           time.Time
	allCounts              map[string]queue.Count
	lggr                   logr.Logger
	status                 PingerStatus

	// prevPodCounts tracks the previous RequestCount per pod per key
	// so we can compute deltas between consecutive polls.
	prevPodCounts map[string]map[string]int64

	// rateBuckets holds per-key windowed ring buffers that accumulate
	// request deltas for rate computation.
	rateBuckets map[string]*queue.RequestsBuckets
}

func newQueuePinger(lggr logr.Logger, getEndpointsFn k8s.GetEndpointsFunc, ns, svcName, deplName, adminPort string) *queuePinger {
	return &queuePinger{
		getEndpointsFn:         getEndpointsFn,
		interceptorNS:          ns,
		interceptorSvcName:     svcName,
		interceptorServiceName: deplName,
		adminPort:              adminPort,
		lggr:                   lggr,
		allCounts:              map[string]queue.Count{},
		prevPodCounts:          map[string]map[string]int64{},
		rateBuckets:            map[string]*queue.RequestsBuckets{},
	}
}

// start starts the queuePinger
func (q *queuePinger) start(ctx context.Context, ticker *time.Ticker) error {
	lggr := q.lggr.WithName("scaler.queuePinger.start")
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			lggr.Error(ctx.Err(), "context marked done. stopping queuePinger loop")
			q.status = PingerERROR
			return fmt.Errorf("context marked done. stopping queuePinger loop: %w", ctx.Err())
		case <-ticker.C:
			err := q.fetchAndSaveCounts(ctx)
			if err != nil {
				lggr.Error(err, "getting request counts")
			}
		}
	}
}

func (q *queuePinger) counts() map[string]queue.Count {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return maps.Clone(q.allCounts)
}

func (q *queuePinger) count(key string) queue.Count {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.allCounts[key]
}

// UpdateBucketConfig sets the window and granularity for a key's rate
// ring buffer. If the config changes, the existing bucket is replaced.
func (q *queuePinger) UpdateBucketConfig(key string, window, granularity time.Duration) {
	q.pingMut.Lock()
	defer q.pingMut.Unlock()
	q.updateBucketConfigLocked(key, window, granularity)
}

func (q *queuePinger) updateBucketConfigLocked(key string, window, granularity time.Duration) {
	if window <= 0 {
		window = defaultWindow
	}
	if granularity <= 0 {
		granularity = defaultGranularity
	}
	if b, ok := q.rateBuckets[key]; ok &&
		b.Window() == window && b.Granularity() == granularity {
		return
	}
	q.rateBuckets[key] = queue.NewRequestsBuckets(window, granularity)
}

// ensureBucketLocked creates a default bucket for key if none exists.
// Must be called with pingMut held.
func (q *queuePinger) ensureBucketLocked(key string) *queue.RequestsBuckets {
	if b, ok := q.rateBuckets[key]; ok {
		return b
	}
	q.updateBucketConfigLocked(key, defaultWindow, defaultGranularity)
	return q.rateBuckets[key]
}

// fetchAndSaveCounts fetches raw counts from all interceptor pods,
// computes per-key request deltas, records them in the windowed
// ring buffers, and updates the aggregated allCounts map.
func (q *queuePinger) fetchAndSaveCounts(ctx context.Context) error {
	q.pingMut.Lock()
	defer q.pingMut.Unlock()

	perPod, err := fetchCountsPerPod(ctx, q.lggr, q.getEndpointsFn, q.interceptorNS, q.interceptorSvcName, q.adminPort)
	if err != nil {
		q.lggr.Error(err, "getting request counts")
		q.status = PingerERROR
		return err
	}
	q.status = PingerACTIVE

	now := time.Now()

	// Per-key aggregated concurrency and request-count delta.
	type keyAgg struct {
		concurrency int
		delta       int64
	}
	agg := make(map[string]keyAgg)

	for podKey, counts := range perPod {
		prev := q.prevPodCounts[podKey]
		newPrev := make(map[string]int64, len(counts.Counts))

		for key, c := range counts.Counts {
			newPrev[key] = c.RequestCount

			ha := agg[key]
			ha.concurrency += c.Concurrency

			if prev != nil {
				if old, ok := prev[key]; ok {
					delta := c.RequestCount - old
					if delta < 0 {
						// Counter reset (pod restarted); treat entire
						// current value as the delta.
						delta = c.RequestCount
					}
					ha.delta += delta
				}
				// New key on an existing pod: skip delta for this
				// tick to avoid a spike.
			}
			// New pod (prev == nil): skip delta for this tick.

			agg[key] = ha
		}
		q.prevPodCounts[podKey] = newPrev
	}

	// Prune pods that are no longer reporting.
	for podKey := range q.prevPodCounts {
		if _, ok := perPod[podKey]; !ok {
			delete(q.prevPodCounts, podKey)
		}
	}

	// Record deltas into ring buffers and compute rates.
	newCounts := make(map[string]queue.Count, len(agg))
	for key, ha := range agg {
		b := q.ensureBucketLocked(key)
		b.Record(now, int(ha.delta))
		newCounts[key] = queue.Count{
			Concurrency: ha.concurrency,
			RPS:         b.WindowAverage(now),
		}
	}

	// Remove buckets for keys that disappeared.
	for key := range q.rateBuckets {
		if _, ok := agg[key]; !ok {
			delete(q.rateBuckets, key)
		}
	}

	q.allCounts = newCounts
	q.lastPingTime = now
	return nil
}

// fetchCountsPerPod fetches counts from every interceptor pod endpoint
// and returns the raw per-pod results keyed by pod URL string.
func fetchCountsPerPod(ctx context.Context, lggr logr.Logger, endpointsFn k8s.GetEndpointsFunc, ns, svcName, adminPort string) (map[string]*queue.Counts, error) {
	lggr = lggr.WithName("queuePinger.requestCounts")

	endpointURLs, err := k8s.EndpointsForService(ctx, ns, svcName, adminPort, endpointsFn)
	if err != nil {
		return nil, err
	}

	if len(endpointURLs) == 0 {
		return nil, fmt.Errorf("there isn't any valid interceptor endpoint")
	}

	type podResult struct {
		key    string
		counts *queue.Counts
	}

	resultCh := make(chan podResult, len(endpointURLs))
	fetchGrp, _ := errgroup.WithContext(ctx)

	for _, endpoint := range endpointURLs {
		u := endpoint
		fetchGrp.Go(func() error {
			counts, err := queue.GetCounts(http.DefaultClient, u)
			if err != nil {
				lggr.Error(err, "getting queue counts from interceptor", "interceptorAddress", u.String())
				return err
			}
			resultCh <- podResult{key: podKey(u), counts: counts}
			return nil
		})
	}

	if err := fetchGrp.Wait(); err != nil {
		lggr.Error(err, "fetching all counts failed")
		return nil, err
	}
	close(resultCh)

	perPod := make(map[string]*queue.Counts, len(endpointURLs))
	for r := range resultCh {
		perPod[r.key] = r.counts
	}
	return perPod, nil
}

func podKey(u url.URL) string {
	return u.Host
}
