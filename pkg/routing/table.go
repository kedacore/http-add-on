package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

var errNotSyncedTable = errors.New("table has not synced")

type Table interface {
	util.HealthChecker

	HasSynced() bool
	Route(req *http.Request) *httpv1alpha1.HTTPScaledObject
	Signal()
	Start(ctx context.Context) error
}

type table struct {
	memoryHolder   util.AtomicValue[*TableMemory]
	memorySignaler util.Signaler
	previousKeys   map[string]struct{}
	reader         client.Reader
	queueCounter   queue.Counter
}

var _ Table = (*table)(nil)

func NewTable(reader client.Reader, counter queue.Counter) Table {
	return &table{
		memorySignaler: util.NewSignaler(),
		previousKeys:   make(map[string]struct{}),
		queueCounter:   counter,
		reader:         reader,
	}
}

func (t *table) refreshMemory(ctx context.Context) error {
	for {
		var list httpv1alpha1.HTTPScaledObjectList
		if err := t.reader.List(ctx, &list); err != nil {
			return fmt.Errorf("listing HTTPScaledObjects: %w", err)
		}

		tm := NewTableMemory()
		currentKeys := make(map[string]struct{})

		for i := range list.Items {
			httpso := &list.Items[i]
			key := fmt.Sprintf("%s/%s", httpso.Namespace, httpso.Name)
			currentKeys[key] = struct{}{}

			tm = tm.Remember(httpso)

			// Ensure queue counter bucket exists
			window := time.Minute
			granularity := time.Second
			if httpso.Spec.ScalingMetric != nil && httpso.Spec.ScalingMetric.Rate != nil {
				window = httpso.Spec.ScalingMetric.Rate.Window.Duration
				granularity = httpso.Spec.ScalingMetric.Rate.Granularity.Duration
			}
			t.queueCounter.UpdateBuckets(key, window, granularity)
		}

		for key := range t.previousKeys {
			if _, exists := currentKeys[key]; !exists {
				t.queueCounter.RemoveKey(key)
			}
		}
		t.previousKeys = currentKeys

		t.memoryHolder.Set(tm)

		if err := t.memorySignaler.Wait(ctx); err != nil {
			return err
		}
	}
}

func (t *table) Signal() {
	t.memorySignaler.Signal()
}

func (t *table) Start(ctx context.Context) error {
	return t.refreshMemory(ctx)
}

func (t *table) Route(req *http.Request) *httpv1alpha1.HTTPScaledObject {
	if req == nil || req.URL == nil {
		return nil
	}

	tm := t.memoryHolder.Get()
	if tm == nil {
		return nil
	}

	hostname := stripPort(req.Host)

	return tm.Route(hostname, req.URL.Path, req.Header)
}

func (t *table) HasSynced() bool {
	tm := t.memoryHolder.Get()
	return tm != nil
}

var _ util.HealthChecker = (*table)(nil)

func (t *table) HealthCheck(_ context.Context) error {
	// TODO: HasSynced never fails after passing once, it is not testing health over time
	if !t.HasSynced() {
		return errNotSyncedTable
	}

	return nil
}
