package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

var errNotSyncedTable = errors.New("table has not synced")

type Table interface {
	util.HealthChecker

	HasSynced() bool
	Route(req *http.Request) *httpv1beta1.InterceptorRoute
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

func (t *table) Route(req *http.Request) *httpv1beta1.InterceptorRoute {
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
