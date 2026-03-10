package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InformerBackedEndpointsCache struct {
	lggr       logr.Logger
	ctrlCache  ctrlcache.Cache
	readyCache *ReadyEndpointsCache

	mu          sync.Mutex
	subscribers map[string]chan struct{}
}

func (i *InformerBackedEndpointsCache) MarshalJSON() ([]byte, error) {
	list := &discov1.EndpointSliceList{}
	if err := i.ctrlCache.List(context.Background(), list); err != nil {
		return nil, err
	}
	items := make([]*discov1.EndpointSlice, len(list.Items))
	for idx := range list.Items {
		items[idx] = &list.Items[idx]
	}
	return json.Marshal(&items)
}

func (i *InformerBackedEndpointsCache) Get(
	ns,
	name string,
) (discov1.EndpointSlice, error) {
	list := &discov1.EndpointSliceList{}
	if err := i.ctrlCache.List(context.Background(), list,
		client.InNamespace(ns),
		client.MatchingLabels{discov1.LabelServiceName: name},
	); err != nil {
		return discov1.EndpointSlice{}, err
	}
	if len(list.Items) == 0 {
		return discov1.EndpointSlice{}, errors.New("no matching endpoints found")
	}
	return list.Items[0], nil
}

// Subscribe returns a buffered channel that receives a signal whenever
// an EndpointSlice owned by the given service changes. Multiple calls
// with the same namespace/serviceName return the same channel.
func (i *InformerBackedEndpointsCache) Subscribe(ns, serviceName string) <-chan struct{} {
	i.mu.Lock()
	defer i.mu.Unlock()
	key := ns + "/" + serviceName
	if ch, ok := i.subscribers[key]; ok {
		return ch
	}
	ch := make(chan struct{}, 1)
	i.subscribers[key] = ch
	return ch
}

// ReadyCache returns the embedded ReadyEndpointsCache, which provides
// O(1) hot-path readiness checks and an efficient broadcast-based
// cold-start wait mechanism.
func (i *InformerBackedEndpointsCache) ReadyCache() *ReadyEndpointsCache {
	return i.readyCache
}

// notifySubscribers sends a non-blocking signal to the subscriber
// channel for the service that owns the given EndpointSlice, if any.
func (i *InformerBackedEndpointsCache) notifySubscribers(slice *discov1.EndpointSlice) {
	svcName := slice.Labels[discov1.LabelServiceName]
	if svcName == "" {
		return
	}
	key := slice.Namespace + "/" + svcName
	i.mu.Lock()
	ch, ok := i.subscribers[key]
	i.mu.Unlock()
	if ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// updateReadyCache updates the ready cache for the service that owns
// the given EndpointSlice.
func (i *InformerBackedEndpointsCache) updateReadyCache(slice *discov1.EndpointSlice) {
	svcName, ok := slice.Labels[discov1.LabelServiceName]
	if !ok || svcName == "" {
		return
	}
	ns := slice.Namespace

	list := &discov1.EndpointSliceList{}
	if err := i.ctrlCache.List(context.Background(), list,
		client.InNamespace(ns),
		client.MatchingLabels{discov1.LabelServiceName: svcName},
	); err != nil {
		i.lggr.Error(err, "failed to list endpoint slices for ready cache update",
			"namespace", ns, "service", svcName)
		return
	}

	allSlices := make([]*discov1.EndpointSlice, len(list.Items))
	for idx := range list.Items {
		allSlices[idx] = &list.Items[idx]
	}
	i.readyCache.Update(ns+"/"+svcName, allSlices)
}

func (i *InformerBackedEndpointsCache) addEvtHandler(obj any) {
	depl, ok := obj.(*discov1.EndpointSlice)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected EndpointSlice, got %v", obj),
			"not forwarding event",
		)
		return
	}

	i.notifySubscribers(depl)
	i.updateReadyCache(depl)
}

func (i *InformerBackedEndpointsCache) updateEvtHandler(_, newObj any) {
	depl, ok := newObj.(*discov1.EndpointSlice)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected EndpointSlice, got %v", newObj),
			"not forwarding event",
		)
		return
	}

	i.notifySubscribers(depl)
	i.updateReadyCache(depl)
}

func (i *InformerBackedEndpointsCache) deleteEvtHandler(obj any) {
	depl, err := endpointSliceFromDeleteObj(obj)
	if err != nil {
		i.lggr.Error(
			err,
			"not forwarding event",
		)
		return
	}

	i.notifySubscribers(depl)
	i.updateReadyCache(depl)
}

// endpointSliceFromDeleteObj unwraps EndpointSlice delete events from either
// a direct object or a DeletedFinalStateUnknown tombstone.
func endpointSliceFromDeleteObj(obj any) (*discov1.EndpointSlice, error) {
	switch t := obj.(type) {
	case *discov1.EndpointSlice:
		return t, nil
	case cache.DeletedFinalStateUnknown:
		depl, ok := t.Obj.(*discov1.EndpointSlice)
		if !ok {
			return nil, fmt.Errorf("informer expected EndpointSlice in tombstone, got %T", t.Obj)
		}
		return depl, nil
	default:
		return nil, fmt.Errorf("informer expected EndpointSlice, got %T", obj)
	}
}

func NewInformerBackedEndpointsCache(
	lggr logr.Logger,
	ctrlCache ctrlcache.Cache,
) (*InformerBackedEndpointsCache, error) {
	informer, err := ctrlCache.GetInformer(context.Background(), &discov1.EndpointSlice{})
	if err != nil {
		lggr.Error(err, "error getting EndpointSlice informer from controller-runtime cache")
		return nil, err
	}

	ret := &InformerBackedEndpointsCache{
		lggr:        lggr,
		ctrlCache:   ctrlCache,
		readyCache:  NewReadyEndpointsCache(lggr),
		subscribers: make(map[string]chan struct{}),
	}

	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ret.addEvtHandler,
		UpdateFunc: ret.updateEvtHandler,
		DeleteFunc: ret.deleteEvtHandler,
	})
	if err != nil {
		lggr.Error(err, "error adding event handler to EndpointSlice informer")
		return nil, err
	}

	return ret, nil
}
