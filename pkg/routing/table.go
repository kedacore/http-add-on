package routing

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	errUnknownSharedIndexInformer = errors.New("informer is not cache.sharedIndexInformer")
	errStartedSharedIndexInformer = errors.New("sharedIndexInformer has started, run more than once is not allowed")
	errStoppedSharedIndexInformer = errors.New("sharedIndexInformer has stopped")
	errNotSyncedTable             = errors.New("table has not synced")
)

type Table interface {
	util.HealthChecker

	Start(ctx context.Context) error
	Route(req *http.Request) *httpv1alpha1.HTTPScaledObject
	HasSynced() bool
}

type table struct {
	httpScaledObjectInformer                 sharedIndexInformer
	httpScaledObjectEventHandlerRegistration cache.ResourceEventHandlerRegistration
	httpScaledObjects                        map[types.NamespacedName]*httpv1alpha1.HTTPScaledObject
	httpScaledObjectsMutex                   sync.RWMutex
	memoryHolder                             util.AtomicValue[TableMemory]
	memorySignaler                           util.Signaler
	queueCounter                             queue.Counter
}

func NewTable(sharedInformerFactory externalversions.SharedInformerFactory, namespace string, counter queue.Counter) (Table, error) {
	httpScaledObjects := informershttpv1alpha1.New(sharedInformerFactory, namespace, nil).HTTPScaledObjects()

	t := table{
		httpScaledObjects: make(map[types.NamespacedName]*httpv1alpha1.HTTPScaledObject),
		memorySignaler:    util.NewSignaler(),
	}

	informer, ok := httpScaledObjects.Informer().(sharedIndexInformer)
	if !ok {
		return nil, errUnknownSharedIndexInformer
	}
	t.httpScaledObjectInformer = informer

	registration, err := informer.AddEventHandler(&t)
	if err != nil {
		return nil, err
	}
	t.httpScaledObjectEventHandlerRegistration = registration
	t.queueCounter = counter
	return &t, nil
}

func (t *table) runInformer(ctx context.Context) error {
	if t.httpScaledObjectInformer.HasStarted() {
		return errStartedSharedIndexInformer
	}

	t.httpScaledObjectInformer.Run(ctx.Done())

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errStoppedSharedIndexInformer
	}
}

func (t *table) refreshMemory(ctx context.Context) error {
	// wait for event handler to be synced before first computation of routes
	for !t.httpScaledObjectEventHandlerRegistration.HasSynced() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			continue
		}
	}

	for {
		m := t.newMemoryFromHTTPSOs()
		t.memoryHolder.Set(m)

		if err := t.memorySignaler.Wait(ctx); err != nil {
			return err
		}
	}
}

func (t *table) newMemoryFromHTTPSOs() TableMemory {
	t.httpScaledObjectsMutex.RLock()
	defer t.httpScaledObjectsMutex.RUnlock()

	tm := NewTableMemory()
	for _, newHTTPSO := range t.httpScaledObjects {
		tm = tm.Remember(newHTTPSO)
	}

	return tm
}

var _ Table = (*table)(nil)

func (t *table) Start(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(util.ApplyContext(t.runInformer, ctx))
	eg.Go(util.ApplyContext(t.refreshMemory, ctx))
	return eg.Wait()
}

func (t *table) Route(req *http.Request) *httpv1alpha1.HTTPScaledObject {
	if req == nil {
		return nil
	}

	tm := t.memoryHolder.Get()
	if tm == nil {
		return nil
	}

	key := NewKeyFromRequest(req)
	return tm.RouteWithHeaders(key, req.Header)
}

func (t *table) HasSynced() bool {
	tm := t.memoryHolder.Get()
	return tm != nil
}

var _ cache.ResourceEventHandler = (*table)(nil)

func (t *table) OnAdd(obj interface{}, _ bool) {
	httpScaledObject, ok := obj.(*httpv1alpha1.HTTPScaledObject)
	if !ok {
		return
	}
	key := *k8s.NamespacedNameFromObject(httpScaledObject)

	window := time.Minute
	granualrity := time.Second
	if httpScaledObject.Spec.ScalingMetric != nil &&
		httpScaledObject.Spec.ScalingMetric.Rate != nil {
		window = httpScaledObject.Spec.ScalingMetric.Rate.Window.Duration
		granualrity = httpScaledObject.Spec.ScalingMetric.Rate.Granularity.Duration
	}
	t.queueCounter.EnsureKey(key.String(), window, granualrity)

	defer t.memorySignaler.Signal()

	t.httpScaledObjectsMutex.Lock()
	defer t.httpScaledObjectsMutex.Unlock()

	t.httpScaledObjects[key] = httpScaledObject
}

func (t *table) OnUpdate(oldObj interface{}, newObj interface{}) {
	oldHTTPSO, ok := oldObj.(*httpv1alpha1.HTTPScaledObject)
	if !ok {
		return
	}
	oldKey := *k8s.NamespacedNameFromObject(oldHTTPSO)

	newHTTPSO, ok := newObj.(*httpv1alpha1.HTTPScaledObject)
	if !ok {
		return
	}
	newKey := *k8s.NamespacedNameFromObject(newHTTPSO)

	window := time.Minute
	granualrity := time.Second
	if newHTTPSO.Spec.ScalingMetric != nil &&
		newHTTPSO.Spec.ScalingMetric.Rate != nil {
		window = newHTTPSO.Spec.ScalingMetric.Rate.Window.Duration
		granualrity = newHTTPSO.Spec.ScalingMetric.Rate.Granularity.Duration
	}
	t.queueCounter.UpdateBuckets(newKey.String(), window, granualrity)

	mustDelete := oldKey != newKey
	defer t.memorySignaler.Signal()

	t.httpScaledObjectsMutex.Lock()
	defer t.httpScaledObjectsMutex.Unlock()

	t.httpScaledObjects[newKey] = newHTTPSO

	if mustDelete {
		delete(t.httpScaledObjects, oldKey)
		t.queueCounter.RemoveKey(oldKey.String())
	}
}

func (t *table) OnDelete(obj interface{}) {
	httpScaledObject, ok := obj.(*httpv1alpha1.HTTPScaledObject)
	if !ok {
		return
	}
	key := *k8s.NamespacedNameFromObject(httpScaledObject)

	defer t.memorySignaler.Signal()

	t.httpScaledObjectsMutex.Lock()
	defer t.httpScaledObjectsMutex.Unlock()

	delete(t.httpScaledObjects, key)

	t.queueCounter.RemoveKey(key.String())
}

var _ util.HealthChecker = (*table)(nil)

func (t *table) HealthCheck(_ context.Context) error {
	if !t.HasSynced() {
		return errNotSyncedTable
	}

	return nil
}
