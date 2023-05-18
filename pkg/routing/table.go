package routing

import (
	"context"
	"net/http"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	listershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/listers/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

var (
	errUnknownSharedIndexInformer = errors.New("The informer is not cache.sharedIndexInformer")
	errStartedSharedIndexInformer = errors.New("The sharedIndexInformer has started, run more than once is not allowed")
	errStoppedSharedIndexInformer = errors.New("The sharedIndexInformer has stopped")
)

type Table interface {
	Start(ctx context.Context) error
	Route(req *http.Request) *httpv1alpha1.HTTPScaledObject
	HasSynced() bool
}

type table struct {
	// TODO(pedrotorres): remove after upgrading k8s.io/client-go to v0.27.0
	httpScaledObjectLister                   listershttpv1alpha1.HTTPScaledObjectLister
	httpScaledObjectInformer                 sharedIndexInformer
	httpScaledObjectEventHandlerRegistration cache.ResourceEventHandlerRegistration
	httpScaledObjects                        map[types.NamespacedName]*httpv1alpha1.HTTPScaledObject
	httpScaledObjectsMutex                   sync.RWMutex
	memoryHolder                             AtomicValue[TableMemory]
	memorySignaler                           Signaler
}

func NewTable(sharedInformerFactory externalversions.SharedInformerFactory, namespace string) (Table, error) {
	httpScaledObjects := informershttpv1alpha1.New(sharedInformerFactory, namespace, nil).HTTPScaledObjects()

	t := table{
		// TODO(pedrotorres): remove after upgrading k8s.io/client-go to v0.27.0
		httpScaledObjectLister: httpScaledObjects.Lister(),
		httpScaledObjects:      make(map[types.NamespacedName]*httpv1alpha1.HTTPScaledObject),
		memorySignaler:         NewSignaler(),
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

	return &t, nil
}

// TODO(pedrotorres): remove after upgrading k8s.io/client-go to v0.27.0
func (t *table) init() error {
	httpScaledObjects, err := t.httpScaledObjectLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, httpScaledObject := range httpScaledObjects {
		t.OnAdd(httpScaledObject)
	}

	return nil
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
	// TODO(pedrotorres): uncomment after upgrading k8s.io/client-go to v0.27.0
	// // wait for event handler to be synced before first computation of routes
	// for !t.httpScaledObjectEventHandlerRegistration.HasSynced() {
	// 	select {
	// 	case <-ctx.Done():
	// 		return ctx.Err()
	// 	case <-time.After(time.Second):
	// 		continue
	// 	}
	// }

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
		namespacedName := k8s.NamespacedNameFromObject(newHTTPSO)
		if oldHTTPSO := tm.Recall(namespacedName); oldHTTPSO != nil {
			// oldest HTTPScaledObject has precedence
			if newHTTPSO.CreationTimestamp.After(oldHTTPSO.CreationTimestamp.Time) {
				continue
			}
		}

		tm = tm.Remember(newHTTPSO)
	}

	return tm
}

var _ Table = (*table)(nil)

func (t *table) Start(ctx context.Context) error {
	// TODO(pedrotorres): remove after upgrading k8s.io/client-go to v0.27.0
	if err := t.init(); err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(applyContext(t.runInformer, ctx))
	eg.Go(applyContext(t.refreshMemory, ctx))
	return eg.Wait()
}

func (t *table) Route(req *http.Request) *httpv1alpha1.HTTPScaledObject {
	if req == nil {
		return nil
	}

	key := NewKeyFromRequest(req)

	tm := t.memoryHolder.Get()
	return tm.Route(key)
}

func (t *table) HasSynced() bool {
	tm := t.memoryHolder.Get()
	return tm != nil
}

var _ cache.ResourceEventHandler = (*table)(nil)

func (t *table) OnAdd(obj interface{}) {
	httpScaledObject, ok := obj.(*httpv1alpha1.HTTPScaledObject)
	if !ok {
		return
	}
	key := *k8s.NamespacedNameFromObject(httpScaledObject)

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

	mustDelete := oldKey != newKey

	defer t.memorySignaler.Signal()

	t.httpScaledObjectsMutex.Lock()
	defer t.httpScaledObjectsMutex.Unlock()

	t.httpScaledObjects[newKey] = newHTTPSO

	if mustDelete {
		delete(t.httpScaledObjects, oldKey)
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
}
