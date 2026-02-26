package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	infdiscov1 "k8s.io/client-go/informers/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InformerBackedEndpointsCache struct {
	lggr                   logr.Logger
	endpointSlicesInformer infdiscov1.EndpointSliceInformer
	bcaster                *watch.Broadcaster
	readyCache             *ReadyEndpointsCache
}

func (i *InformerBackedEndpointsCache) MarshalJSON() ([]byte, error) {
	lst := i.endpointSlicesInformer.Lister()
	depls, err := lst.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return json.Marshal(&depls)
}

func (i *InformerBackedEndpointsCache) Start(ctx context.Context) {
	i.endpointSlicesInformer.Informer().Run(ctx.Done())
}

func (i *InformerBackedEndpointsCache) Get(
	ns,
	name string,
) (discov1.EndpointSlice, error) {
	req, err := labels.NewRequirement(discov1.LabelServiceName, selection.Equals, []string{name})
	if err != nil {
		return discov1.EndpointSlice{}, err
	}
	ls := labels.NewSelector().Add(*req)
	depl, err := i.endpointSlicesInformer.Lister().EndpointSlices(ns).List(ls)
	if err != nil {
		return discov1.EndpointSlice{}, err
	}
	if len(depl) == 0 {
		return discov1.EndpointSlice{}, errors.New("no matching endpoints found")
	}
	return *depl[0], nil
}

func (i *InformerBackedEndpointsCache) Watch(
	ns,
	name string,
) (watch.Interface, error) {
	watched, err := i.bcaster.Watch()
	if err != nil {
		return nil, err
	}
	return watch.Filter(watched, func(e watch.Event) (watch.Event, bool) {
		depl := e.Object.(*discov1.EndpointSlice)
		if depl.Namespace == ns && depl.GetLabels()[discov1.LabelServiceName] == name {
			return e, true
		}
		return e, false
	}), nil
}

// ReadyCache returns the embedded ReadyEndpointsCache, which provides
// O(1) hot-path readiness checks and an efficient broadcast-based
// cold-start wait mechanism.
func (i *InformerBackedEndpointsCache) ReadyCache() *ReadyEndpointsCache {
	return i.readyCache
}

// updateReadyCache updates the ready cache for the service that owns
// the given EndpointSlice.
func (i *InformerBackedEndpointsCache) updateReadyCache(slice *discov1.EndpointSlice) {
	svcName, ok := slice.Labels[discov1.LabelServiceName]
	if !ok || svcName == "" {
		return
	}
	ns := slice.Namespace

	req, err := labels.NewRequirement(discov1.LabelServiceName, selection.Equals, []string{svcName})
	if err != nil {
		return
	}
	sel := labels.NewSelector().Add(*req)
	allSlices, err := i.endpointSlicesInformer.Lister().EndpointSlices(ns).List(sel)
	if err != nil {
		i.lggr.Error(err, "failed to list endpoint slices for ready cache update",
			"namespace", ns, "service", svcName)
		return
	}

	i.readyCache.Update(ns+"/"+svcName, allSlices)
}

func (i *InformerBackedEndpointsCache) addEvtHandler(obj any) {
	depl, ok := obj.(*discov1.EndpointSlice)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected service, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Added, depl); err != nil {
		i.lggr.Error(err, "informer expected service")
	}
	i.updateReadyCache(depl)
}

func (i *InformerBackedEndpointsCache) updateEvtHandler(_, newObj any) {
	depl, ok := newObj.(*discov1.EndpointSlice)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected service, got %v", newObj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Modified, depl); err != nil {
		i.lggr.Error(err, "informer expected service")
	}
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

	if err := i.bcaster.Action(watch.Deleted, depl); err != nil {
		i.lggr.Error(err, "informer expected service")
	}
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

// TODO: migrate from client-go SharedInformerFactory to controller-runtime
// cache, which is already used for HTTPScaledObject watching. This would
// eliminate the separate informer lifecycle and kubernetes.Interface dependency.
func NewInformerBackedEndpointsCache(
	lggr logr.Logger,
	cl kubernetes.Interface,
	defaultResync time.Duration,
) *InformerBackedEndpointsCache {
	factory := informers.NewSharedInformerFactory(
		cl,
		defaultResync,
	)
	endpointSlicesInformer := factory.Discovery().V1().EndpointSlices()
	ret := &InformerBackedEndpointsCache{
		lggr:                   lggr,
		bcaster:                watch.NewBroadcaster(0, watch.WaitIfChannelFull),
		endpointSlicesInformer: endpointSlicesInformer,
		readyCache:             NewReadyEndpointsCache(lggr),
	}
	_, err := ret.endpointSlicesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ret.addEvtHandler,
		UpdateFunc: ret.updateEvtHandler,
		DeleteFunc: ret.deleteEvtHandler,
	})
	if err != nil {
		lggr.Error(err, "error creating backend informer")
	}
	return ret
}
