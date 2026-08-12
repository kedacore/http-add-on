package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	errNotSyncedTable = errors.New("table has not synced")
	errStaleTable     = errors.New("table refresh loop has stalled")
)

type Table interface {
	util.HealthChecker

	HasSynced() bool
	Route(req *http.Request) *httpv1beta1.InterceptorRoute
	Signal()
	Start(ctx context.Context) error
}

// HealthConfig configures the table's periodic self-rebuild and the staleness
// deadline enforced by HealthCheck.
//
// The deadline detects a refresh loop that has stopped making progress. It does
// not detect a stale informer cache: once an informer has completed its initial
// sync, reads from it return immediately whether or not its watch is still
// healthy, so a rebuild can succeed against data that no longer reflects the
// cluster.
type HealthConfig struct {
	// RefreshInterval is how often the table rebuilds itself, in addition to
	// rebuilds triggered by Signal. The periodic rebuild keeps the last
	// rebuild time advancing even when no informer events arrive — an
	// informer watching zero objects delivers no resync events — which is
	// what allows HealthCheck to detect a stalled refresh loop without
	// false positives on an empty table. Zero disables periodic rebuilds.
	RefreshInterval time.Duration

	// MaxStaleness is how long the table may go without a successful rebuild
	// before HealthCheck fails. It should be at least twice RefreshInterval so
	// that a single missed rebuild is tolerated, and it needs RefreshInterval
	// to be set: with periodic rebuilds disabled, an interceptor watching zero
	// routes has nothing to advance its rebuild time and would fail the check
	// as soon as MaxStaleness elapsed. Zero disables the check.
	MaxStaleness time.Duration

	// OnRebuild, if set, is called with the timestamp of every successful
	// rebuild. It lets callers export the rebuild time without the routing
	// table depending on a metrics package. It runs on the refresh loop, so it
	// must not block.
	OnRebuild func(time.Time)
}

type table struct {
	memoryHolder   util.AtomicValue[*TableMemory]
	memorySignaler util.Signaler
	previousKeys   map[string]struct{}
	reader         client.Reader
	queueCounter   queue.Counter

	refreshInterval time.Duration
	maxStaleness    time.Duration
	onRebuild       func(time.Time)
	lastRebuild     atomic.Pointer[time.Time]
}

var _ Table = (*table)(nil)

func NewTable(reader client.Reader, counter queue.Counter, health HealthConfig) Table {
	return &table{
		memorySignaler:  util.NewSignaler(),
		previousKeys:    make(map[string]struct{}),
		queueCounter:    counter,
		reader:          reader,
		refreshInterval: health.RefreshInterval,
		maxStaleness:    health.MaxStaleness,
		onRebuild:       health.OnRebuild,
	}
}

func (t *table) refreshMemory(ctx context.Context) error {
	for {
		var irList httpv1beta1.InterceptorRouteList
		if err := t.reader.List(ctx, &irList); err != nil {
			return fmt.Errorf("failed to list InterceptorRoutes: %w", err)
		}

		tm := NewTableMemory()
		currentKeys := make(map[string]struct{})

		for i := range irList.Items {
			ir := &irList.Items[i]
			key := k8s.ResourceKey(ir.Namespace, ir.Name)

			currentKeys[key] = struct{}{}

			tm = tm.Remember(ir)

			t.queueCounter.EnsureKey(key)
		}

		// TODO(v1): remove the HTTPSO to IR conversion
		var httpsoList httpv1alpha1.HTTPScaledObjectList
		if err := t.reader.List(ctx, &httpsoList); err != nil {
			return fmt.Errorf("listing HTTPScaledObjects: %w", err)
		}

		for i := range httpsoList.Items {
			httpso := &httpsoList.Items[i]
			key := fmt.Sprintf("%s/%s", httpso.Namespace, httpso.Name)

			if _, ok := currentKeys[key]; ok {
				// skip the conflicting HTTPSO, IR takes precedence
				continue
			}
			currentKeys[key] = struct{}{}

			// Create an IR from the HTTPSO to simplify the whole routing logic
			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: httpso.CreationTimestamp,
					Name:              httpso.Name,
					Namespace:         httpso.Namespace,
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Port:     httpso.Spec.ScaleTargetRef.Port,
						PortName: httpso.Spec.ScaleTargetRef.PortName,
						Service:  httpso.Spec.ScaleTargetRef.Service,
					},
				},
			}

			rr := httpv1beta1.RoutingRule{
				Hosts: httpso.Spec.Hosts,
			}
			for _, pathPrefix := range httpso.Spec.PathPrefixes {
				rr.Paths = append(rr.Paths, httpv1beta1.PathMatch{
					Value: pathPrefix,
				})
			}
			for _, header := range httpso.Spec.Headers {
				rr.Headers = append(rr.Headers, httpv1beta1.HeaderMatch{
					Name:  header.Name,
					Value: header.Value,
				})
			}
			ir.Spec.Rules = []httpv1beta1.RoutingRule{rr}

			// Convert HTTPSO timeouts to InterceptorRoute timeouts spec.
			if httpso.Spec.Timeouts != nil {
				if httpso.Spec.Timeouts.ConditionWait.Duration > 0 {
					ir.Spec.Timeouts.Readiness = &metav1.Duration{Duration: httpso.Spec.Timeouts.ConditionWait.Duration}
				}
				if httpso.Spec.Timeouts.ResponseHeader.Duration > 0 {
					ir.Spec.Timeouts.ResponseHeader = &metav1.Duration{Duration: httpso.Spec.Timeouts.ResponseHeader.Duration}
				}
			}

			if c := httpso.Spec.ColdStartTimeoutFailoverRef; c != nil {
				ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
					Fallback: &httpv1beta1.ColdStartFallback{
						Service: &httpv1beta1.ServiceRef{
							Name:     c.Service,
							Port:     c.Port,
							PortName: c.PortName,
						},
					},
				}
				if c.TimeoutSeconds > 0 {
					ir.Spec.Timeouts.Readiness = &metav1.Duration{Duration: time.Duration(c.TimeoutSeconds) * time.Second}
				}
			}

			if httpso.Spec.ScalingMetric != nil {
				if httpso.Spec.ScalingMetric.Concurrency != nil {
					ir.Spec.ScalingMetric.Concurrency = &httpv1beta1.ConcurrencyTargetSpec{
						TargetValue: int32(httpso.Spec.ScalingMetric.Concurrency.TargetValue), //nolint:gosec // kubebuilder-validated field, overflow not possible
					}
				}
				if httpso.Spec.ScalingMetric.Rate != nil {
					ir.Spec.ScalingMetric.RequestRate = &httpv1beta1.RequestRateTargetSpec{
						TargetValue: int32(httpso.Spec.ScalingMetric.Rate.TargetValue), //nolint:gosec // kubebuilder-validated field, overflow not possible
						Window:      httpso.Spec.ScalingMetric.Rate.Window,
						Granularity: httpso.Spec.ScalingMetric.Rate.Granularity,
					}
				}
			}

			tm = tm.Remember(ir)

			t.queueCounter.EnsureKey(key)
		}

		for key := range t.previousKeys {
			if _, exists := currentKeys[key]; !exists {
				t.queueCounter.RemoveKey(key)
			}
		}
		t.previousKeys = currentKeys

		// Recorded before publishing, so HasSynced can never report a synced
		// table with no rebuild timestamp behind it.
		now := time.Now()
		t.lastRebuild.Store(&now)

		t.memoryHolder.Set(tm)

		if t.onRebuild != nil {
			t.onRebuild(now)
		}

		if err := t.memorySignaler.Wait(ctx); err != nil {
			return err
		}
	}
}

func (t *table) Signal() {
	t.memorySignaler.Signal()
}

func (t *table) Start(ctx context.Context) error {
	if t.refreshInterval > 0 {
		go t.periodicSignal(ctx)
	}

	return t.refreshMemory(ctx)
}

// periodicSignal nudges the refresh loop on a fixed cadence so the table is
// rebuilt even when no informer events arrive. Without it, a table watching
// zero route objects would never rebuild after the initial sync, and the
// staleness check in HealthCheck could not tell a wedged refresh loop apart
// from a legitimately quiet one.
func (t *table) periodicSignal(ctx context.Context) {
	ticker := time.NewTicker(t.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.memorySignaler.Signal()
		}
	}
}

func (t *table) Route(req *http.Request) *httpv1beta1.InterceptorRoute {
	if req == nil || req.URL == nil {
		return nil
	}

	tm := t.memoryHolder.Get()
	if tm == nil {
		return nil
	}

	hostname := StripPort(req.Host)

	return tm.Route(hostname, req.URL.Path, req.Header)
}

func (t *table) HasSynced() bool {
	tm := t.memoryHolder.Get()
	return tm != nil
}

var _ util.HealthChecker = (*table)(nil)

func (t *table) HealthCheck(_ context.Context) error {
	if !t.HasSynced() {
		return errNotSyncedTable
	}

	if t.maxStaleness > 0 {
		if last := t.lastRebuild.Load(); last != nil {
			if age := time.Since(*last); age > t.maxStaleness {
				return fmt.Errorf("%w: last successful rebuild was %s ago (limit %s)", errStaleTable, age.Truncate(time.Second), t.maxStaleness)
			}
		}
	}

	return nil
}
