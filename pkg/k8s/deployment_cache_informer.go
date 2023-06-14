package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	infappsv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InformerBackedDeploymentCache struct {
	lggr         logr.Logger
	deplInformer infappsv1.DeploymentInformer
	bcaster      *watch.Broadcaster
}

func (i *InformerBackedDeploymentCache) MarshalJSON() ([]byte, error) {
	lst := i.deplInformer.Lister()
	depls, err := lst.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return json.Marshal(&depls)
}

func (i *InformerBackedDeploymentCache) Start(ctx context.Context) error {
	i.deplInformer.Informer().Run(ctx.Done())
	return errors.Wrap(
		ctx.Err(), "deployment cache informer was stopped",
	)
}

func (i *InformerBackedDeploymentCache) Get(
	ns,
	name string,
) (appsv1.Deployment, error) {
	depl, err := i.deplInformer.Lister().Deployments(ns).Get(name)
	if err != nil {
		return appsv1.Deployment{}, err
	}
	return *depl, nil
}

func (i *InformerBackedDeploymentCache) Watch(
	ns,
	name string,
) (watch.Interface, error) {
	watched, err := i.bcaster.Watch()
	if err != nil {
		return nil, err
	}
	return watch.Filter(watched, func(e watch.Event) (watch.Event, bool) {
		depl := e.Object.(*appsv1.Deployment)
		if depl.Namespace == ns && depl.Name == name {
			return e, true
		}
		return e, false
	}), nil
}

func (i *InformerBackedDeploymentCache) addEvtHandler(obj interface{}) {
	depl, ok := obj.(*appsv1.Deployment)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected deployment, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Added, depl); err != nil {
		i.lggr.Error(err, "informer expected deployment")
	}
}

func (i *InformerBackedDeploymentCache) updateEvtHandler(_, newObj interface{}) {
	depl, ok := newObj.(*appsv1.Deployment)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected deployment, got %v", newObj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Modified, depl); err != nil {
		i.lggr.Error(err, "informer expected deployment")
	}
}

func (i *InformerBackedDeploymentCache) deleteEvtHandler(obj interface{}) {
	depl, ok := obj.(*appsv1.Deployment)
	if !ok {
		i.lggr.Error(
			fmt.Errorf("informer expected deployment, got %v", obj),
			"not forwarding event",
		)
		return
	}

	if err := i.bcaster.Action(watch.Deleted, depl); err != nil {
		i.lggr.Error(err, "informer expected deployment")
	}
}

func NewInformerBackedDeploymentCache(
	lggr logr.Logger,
	cl kubernetes.Interface,
	defaultResync time.Duration,
) *InformerBackedDeploymentCache {
	factory := informers.NewSharedInformerFactory(
		cl,
		defaultResync,
	)
	deplInformer := factory.Apps().V1().Deployments()
	ret := &InformerBackedDeploymentCache{
		lggr:         lggr,
		bcaster:      watch.NewBroadcaster(0, watch.WaitIfChannelFull),
		deplInformer: deplInformer,
	}
	_, err := ret.deplInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ret.addEvtHandler,
		UpdateFunc: ret.updateEvtHandler,
		DeleteFunc: ret.deleteEvtHandler,
	})
	if err != nil {
		lggr.Error(err, "error creating backend informer")
	}
	return ret
}
