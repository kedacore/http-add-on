package k8s

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	infappsv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InformerBackedDeploymentCache struct {
	sei         infappsv1.DeploymentInformer
	bcastersMut *sync.RWMutex
	// bcasters is a map of broadcasters for each deployment
	bcasters map[client.ObjectKey]*watch.Broadcaster
}

func (i *InformerBackedDeploymentCache) Start(ctx context.Context) error {
	i.sei.Informer().Run(ctx.Done())
	return errors.Wrap(
		ctx.Err(), "deployment cache informer was stopped",
	)
}

func (i *InformerBackedDeploymentCache) Get(
	ns,
	name string,
) (appsv1.Deployment, error) {
	depl, err := i.sei.Lister().Deployments(ns).Get(name)
	if err != nil {
		return appsv1.Deployment{}, err
	}
	return *depl, nil
}

func (i *InformerBackedDeploymentCache) Watch(
	ns,
	name string,
) watch.Interface {
	i.bcastersMut.Lock()
	defer i.bcastersMut.Unlock()
	key := client.ObjectKey{Namespace: ns, Name: name}
	bcaster, ok := i.bcasters[key]
	if !ok {
		bcaster = watch.NewBroadcaster(
			0,
			watch.WaitIfChannelFull,
		)
		i.bcasters[key] = bcaster
	}
	return bcaster.Watch()
}

func (i *InformerBackedDeploymentCache) addEvtHandler(obj interface{}) {
	i.bcastersMut.Lock()
	defer i.bcastersMut.Unlock()
}

func (i *InformerBackedDeploymentCache) updateEvtHandler(obj interface{}) {
	i.bcastersMut.Lock()
	defer i.bcastersMut.Unlock()
}

func (i *InformerBackedDeploymentCache) deleteEvtHandler(obj interface{}) {
	i.bcastersMut.Lock()
	defer i.bcastersMut.Unlock()
}

func NewInformerBackedDeploymentCache(
	cl kubernetes.Interface,
	defaultResync time.Duration,
) *InformerBackedDeploymentCache {
	factory := informers.NewSharedInformerFactory(
		cl,
		defaultResync,
	)
	deplInformer := factory.Apps().V1().Deployments()
	ret := &InformerBackedDeploymentCache{
		bcastersMut: new(sync.RWMutex),
		bcasters:    map[client.ObjectKey]*watch.Broadcaster{},
		sei:         deplInformer,
	}
	ret.sei.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		OnAdd:    ret.addEvtHandler,
		OnUpdate: ret.updateEvtHandler,
		OnDelete: ret.deleteEvtHandler,
	})
}
