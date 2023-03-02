package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	infcorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InformerConfigMapUpdater struct {
	lggr       logr.Logger
	cmInformer infcorev1.ConfigMapInformer
	bcaster    *watch.Broadcaster
}

func (i *InformerConfigMapUpdater) MarshalJSON() ([]byte, error) {
	lst := i.cmInformer.Lister()
	cms, err := lst.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return json.Marshal(&cms)
}

func (i *InformerConfigMapUpdater) Start(ctx context.Context) error {
	i.cmInformer.Informer().Run(ctx.Done())
	return errors.Wrap(
		ctx.Err(),
		"configMap informer was stopped",
	)
}

func (i *InformerConfigMapUpdater) Get(
	ns,
	name string,
) (corev1.ConfigMap, error) {
	cm, err := i.cmInformer.Lister().ConfigMaps(ns).Get(name)
	if err != nil {
		return corev1.ConfigMap{}, err
	}
	return *cm, nil
}

func (i *InformerConfigMapUpdater) Watch(
	ns,
	name string,
) (watch.Interface, error) {
	watched, err := i.bcaster.Watch()
	if err != nil {
		return nil, err
	}
	return watch.Filter(watched, func(e watch.Event) (watch.Event, bool) {
		cm, ok := e.Object.(*corev1.ConfigMap)
		if !ok {
			i.lggr.Error(
				fmt.Errorf("informer expected ConfigMap, ignoring this event"),
				"event",
				e,
			)
			return e, false
		}
		if cm.Namespace == ns && cm.Name == name {
			return e, true
		}
		return e, false
	}), nil
}

func (i *InformerConfigMapUpdater) addEvtHandler(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected configMap, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Added, cm); err != nil {
		i.lggr.Error(err, "informer expected configMap")
	}
}

func (i *InformerConfigMapUpdater) updateEvtHandler(oldObj, newObj interface{}) {
	cm, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected configMap, got %v", newObj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Modified, cm); err != nil {
		i.lggr.Error(err, "informer expected configMap")
	}
}

func (i *InformerConfigMapUpdater) deleteEvtHandler(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected configMap, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Deleted, cm); err != nil {
		i.lggr.Error(err, "informer expected configMap")
	}
}

func NewInformerConfigMapUpdater(
	lggr logr.Logger,
	cl kubernetes.Interface,
	defaultResync time.Duration,
	namespace string,
) *InformerConfigMapUpdater {
	factory := informers.NewSharedInformerFactoryWithOptions(
		cl,
		defaultResync,
		informers.WithNamespace(namespace),
	)
	cmInformer := factory.Core().V1().ConfigMaps()
	ret := &InformerConfigMapUpdater{
		lggr:       lggr,
		bcaster:    watch.NewBroadcaster(0, watch.WaitIfChannelFull),
		cmInformer: cmInformer,
	}
	_, err := ret.cmInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ret.addEvtHandler,
		UpdateFunc: ret.updateEvtHandler,
		DeleteFunc: ret.deleteEvtHandler,
	})
	if err != nil {
		lggr.Error(err, "error creating config informer")
	}
	return ret
}
