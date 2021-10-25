package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type DeploymentCache interface {
	json.Marshaler
	Get(name string) (appsv1.Deployment, error)
	Watch(name string) watch.Interface
}

type K8sDeploymentCache struct {
	latest      map[string]appsv1.Deployment
	rwm         *sync.RWMutex
	cl          DeploymentListerWatcher
	broadcaster *watch.Broadcaster
}

func NewK8sDeploymentCache(
	ctx context.Context,
	lggr logr.Logger,
	cl DeploymentListerWatcher,
) (*K8sDeploymentCache, error) {
	lggr = lggr.WithName("pkg.k8s.NewK8sDeploymentCache")
	bcaster := watch.NewBroadcaster(5, watch.DropIfChannelFull)

	ret := &K8sDeploymentCache{
		latest:      map[string]appsv1.Deployment{},
		rwm:         new(sync.RWMutex),
		broadcaster: bcaster,
		cl:          cl,
	}
	deployList, err := cl.List(ctx, metav1.ListOptions{})
	if err != nil {
		lggr.Error(
			err,
			"failed to fetch initial deployment list",
		)
		return nil, err
	}
	ret.mergeAndBroadcastList(deployList)
	return ret, nil
}

func (k *K8sDeploymentCache) MarshalJSON() ([]byte, error) {
	k.rwm.RLock()
	defer k.rwm.RUnlock()
	ret := map[string]int32{}
	for name, depl := range k.latest {
		ret[name] = depl.Status.ReadyReplicas
	}
	return json.Marshal(ret)
}

func (k *K8sDeploymentCache) StartWatcher(
	ctx context.Context,
	lggr logr.Logger,
	fetchTickDur time.Duration,
) error {
	lggr = lggr.WithName(
		"pkg.k8s.K8sDeploymentCache.StartWatcher",
	)
	watcher, err := k.cl.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		lggr.Error(
			err,
			"couldn't create new watch stream",
		)
		return errors.Wrap(
			err,
			"error creating new watch stream",
		)
	}

	ch := watcher.ResultChan()
	fetchTicker := time.NewTicker(fetchTickDur)
	defer fetchTicker.Stop()
	for {
		select {
		case <-fetchTicker.C:
			deplList, err := k.cl.List(ctx, metav1.ListOptions{})
			if err != nil {
				lggr.Error(
					err,
					"error with periodic deployment fetch",
				)
				return errors.Wrap(
					err,
					"error with periodic deployment fetch",
				)
			}
			k.mergeAndBroadcastList(deplList)
		case evt, validRecv := <-ch:
			// handle closed watch stream
			if !validRecv {
				newWatcher, err := k.cl.Watch(ctx, metav1.ListOptions{})
				if err != nil {
					lggr.Error(
						err,
						"watch stream was closed and couldn't re-open it",
					)
					return errors.Wrap(
						err,
						"failed to re-open watch stream",
					)
				}
				ch = newWatcher.ResultChan()
			} else {
				if err := k.addEvt(evt); err != nil {
					lggr.Error(
						err,
						"couldn't add event to the deployment cache",
					)
					return errors.Wrap(
						err,
						"error adding event to the deployment cache",
					)
				}
				k.broadcaster.Action(evt.Type, evt.Object)
			}
		case <-ctx.Done():
			lggr.Error(
				ctx.Err(),
				"context is done",
			)
			return errors.Wrap(
				ctx.Err(),
				"context is marked done",
			)
		}
	}
}

// mergeList adds each deployment in lst to the internal
// list of events and broadcasts a new event for each
// one.
func (k *K8sDeploymentCache) mergeAndBroadcastList(
	lst *appsv1.DeploymentList,
) {
	k.rwm.Lock()
	defer k.rwm.Unlock()
	for _, depl := range lst.Items {
		k.latest[depl.ObjectMeta.Name] = depl
		// if the deployment isn't already in the cache,
		// we need to broadcast an ADDED event, otherwise
		// broadcast a MODIFIED event
		_, ok := k.latest[depl.ObjectMeta.Name]
		evtType := watch.Modified
		if !ok {
			evtType = watch.Added
		}

		k.broadcaster.Action(evtType, &depl)
	}
}

// addEvt checks to make sure evt.Object is an actual
// Deployment. if it isn't, returns a descriptive error.
// otherwise, adds evt to the internal events list
func (k *K8sDeploymentCache) addEvt(evt watch.Event) error {
	k.rwm.Lock()
	defer k.rwm.Unlock()
	depl, ok := evt.Object.(*appsv1.Deployment)
	// if we didn't get back a deployment in the event,
	// something is wrong that we can't fix, so just continue
	if !ok {
		return fmt.Errorf(
			"watch event did not contain a Deployment",
		)
	}
	k.latest[depl.GetObjectMeta().GetName()] = *depl
	return nil
}

func (k *K8sDeploymentCache) Get(name string) (appsv1.Deployment, error) {
	k.rwm.RLock()
	defer k.rwm.RUnlock()
	depl, ok := k.latest[name]
	if !ok {
		return appsv1.Deployment{}, fmt.Errorf("no deployment %s found", name)
	}
	return depl, nil
}

func (k *K8sDeploymentCache) Watch(name string) watch.Interface {
	watcher := k.broadcaster.Watch()
	return watch.Filter(watcher, func(evt watch.Event) (watch.Event, bool) {
		depl, ok := evt.Object.(*appsv1.Deployment)
		if !ok {
			return evt, false
		}
		return evt, depl.ObjectMeta.Name == name
	})
}

// MemoryDeploymentCache is a purely in-memory DeploymentCache implementation.
//
// To ensure this is concurrency-safe, be sure to use RWM properly to protect
// all accesses to either map in this struct.
type MemoryDeploymentCache struct {
	// RWM protects all accesses to either of Watchers or Deployments
	RWM *sync.RWMutex

	// Watchers holds watchers to be returned by calls to Watch. If Watch is called with a
	// name that has a key in this map, that function will panic. Otherwise, it will
	// return the corresponding value
	Watchers map[string]*watch.RaceFreeFakeWatcher

	// Deployments holds the deployments to be returned in calls to Get. If Get is called
	// with a name that exists as a key in this map, the corresponding value will be returned.
	// Otherwise, an error will be returned
	Deployments map[string]appsv1.Deployment
}

// NewMemoryDeploymentCache creates a new MemoryDeploymentCache with the Deployments map set to
// initialDeployments, and the Watchers map initialized with a newly created and otherwise
// untouched FakeWatcher for each key in the initialDeployments map
func NewMemoryDeploymentCache(
	initialDeployments map[string]appsv1.Deployment,
) *MemoryDeploymentCache {
	ret := &MemoryDeploymentCache{
		RWM:         new(sync.RWMutex),
		Watchers:    make(map[string]*watch.RaceFreeFakeWatcher),
		Deployments: make(map[string]appsv1.Deployment),
	}
	ret.Deployments = initialDeployments
	for deployName := range initialDeployments {
		ret.Watchers[deployName] = watch.NewRaceFreeFake()
	}
	return ret
}

func (m *MemoryDeploymentCache) MarshalJSON() ([]byte, error) {
	m.RWM.RLock()
	defer m.RWM.RUnlock()
	ret := map[string]int32{}
	for name, depl := range m.Deployments {
		ret[name] = depl.Status.ReadyReplicas
	}
	return json.Marshal(ret)
}

func (m *MemoryDeploymentCache) Get(name string) (appsv1.Deployment, error) {
	m.RWM.RLock()
	defer m.RWM.RUnlock()
	val, ok := m.Deployments[name]
	if !ok {
		return appsv1.Deployment{}, fmt.Errorf(
			"deployment %s not found",
			name,
		)
	}
	return val, nil
}

func (m *MemoryDeploymentCache) Watch(name string) watch.Interface {
	m.RWM.RLock()
	defer m.RWM.RUnlock()
	val, ok := m.Watchers[name]
	if !ok {
		errString := fmt.Sprintf(
			"(github.com/kedacore/http-add-on/pkg/k8s).MemoryDeploymentCacher.Watch(%s) called, but that name doesn't exist in watchers map",
			name,
		)
		panic(errString)
	}
	return val
}
