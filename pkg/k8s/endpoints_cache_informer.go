package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	infcorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InformerBackedEndpointsCache struct {
	lggr              logr.Logger
	endpointsInformer infcorev1.EndpointsInformer
	bcaster           *watch.Broadcaster
}

func (i *InformerBackedEndpointsCache) MarshalJSON() ([]byte, error) {
	lst := i.endpointsInformer.Lister()
	depls, err := lst.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return json.Marshal(&depls)
}

func (i *InformerBackedEndpointsCache) Start(ctx context.Context) {
	i.endpointsInformer.Informer().Run(ctx.Done())
}

func (i *InformerBackedEndpointsCache) Get(
	ns,
	name string,
) (v1.Endpoints, error) {
	depl, err := i.endpointsInformer.Lister().Endpoints(ns).Get(name)
	if err != nil {
		return v1.Endpoints{}, err
	}
	return *depl, nil
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
		depl := e.Object.(*v1.Endpoints)
		if depl.Namespace == ns && depl.Name == name {
			return e, true
		}
		return e, false
	}), nil
}

func (i *InformerBackedEndpointsCache) addEvtHandler(obj interface{}) {
	depl, ok := obj.(*v1.Endpoints)
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
}

func (i *InformerBackedEndpointsCache) updateEvtHandler(_, newObj interface{}) {
	depl, ok := newObj.(*v1.Endpoints)
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
}

func (i *InformerBackedEndpointsCache) deleteEvtHandler(obj interface{}) {
	depl, ok := obj.(*v1.Endpoints)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected service, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Deleted, depl); err != nil {
		i.lggr.Error(err, "informer expected service")
	}
}

func NewInformerBackedEndpointsCache(
	lggr logr.Logger,
	cl kubernetes.Interface,
	defaultResync time.Duration,
) *InformerBackedEndpointsCache {
	factory := informers.NewSharedInformerFactory(
		cl,
		defaultResync,
	)
	endpointsInformer := factory.Core().V1().Endpoints()
	ret := &InformerBackedEndpointsCache{
		lggr:              lggr,
		bcaster:           watch.NewBroadcaster(0, watch.WaitIfChannelFull),
		endpointsInformer: endpointsInformer,
	}
	_, err := ret.endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ret.addEvtHandler,
		UpdateFunc: ret.updateEvtHandler,
		DeleteFunc: ret.deleteEvtHandler,
	})
	if err != nil {
		lggr.Error(err, "error creating backend informer")
	}
	return ret
}
