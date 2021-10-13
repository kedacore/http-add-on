package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// closeableWatcher is a watch.Interface that, when closed
// will be reopened
type closeableWatcher struct {
	uid         uuid.UUID
	mut         *sync.RWMutex
	ch          chan watch.Event
	events      []watch.Event
	closed      bool
	allowReopen bool
}

func newCloseableWatcher() *closeableWatcher {
	return &closeableWatcher{
		uid:         uuid.New(),
		mut:         new(sync.RWMutex),
		ch:          make(chan watch.Event),
		closed:      false,
		allowReopen: true,
	}
}

func (w *closeableWatcher) String() string {
	return fmt.Sprintf(
		"closeableWatcher %s. events = %v",
		w.uid.String(),
		w.events,
	)
}

func (w *closeableWatcher) Stop() {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if !w.closed {
		close(w.ch)
		w.closed = true
	}
}

func (w *closeableWatcher) ResultChan() <-chan watch.Event {
	w.mut.Lock()
	defer w.mut.Unlock()
	if w.closed && w.allowReopen {
		w.ch = make(chan watch.Event)
		w.closed = false
	}
	return w.ch
}

func (w *closeableWatcher) closeOpenChans(allowReopen bool) {
	w.mut.Lock()
	defer w.mut.Unlock()
	close(w.ch)
	w.closed = true
	w.allowReopen = allowReopen
}

func (w *closeableWatcher) Add(d *appsv1.Deployment) error {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if w.closed {
		return errors.New("watcher is closed")
	}
	evt := watch.Event{
		Type:   watch.Added,
		Object: d,
	}
	w.ch <- evt
	w.events = append(w.events, evt)
	return nil
}

func (w *closeableWatcher) Modify(d *appsv1.Deployment) error {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if w.closed {
		return errors.New("watcher is closed")
	}
	evt := watch.Event{
		Type:   watch.Modified,
		Object: d,
	}
	w.ch <- evt
	w.events = append(w.events, evt)
	return nil
}

func (w *closeableWatcher) getEvents() []watch.Event {
	w.mut.RLock()
	defer w.mut.RUnlock()
	return w.events
}

type fakeDeploymentListerWatcher struct {
	mut     *sync.RWMutex
	watcher *closeableWatcher
	items   map[string]appsv1.Deployment
}

func newFakeDeploymentListerWatcher() *fakeDeploymentListerWatcher {
	w := newCloseableWatcher()
	return &fakeDeploymentListerWatcher{
		mut:     new(sync.RWMutex),
		watcher: w,
		items:   map[string]appsv1.Deployment{},
	}
}

func (lw *fakeDeploymentListerWatcher) List(ctx context.Context, options metav1.ListOptions) (*appsv1.DeploymentList, error) {
	lw.mut.Lock()
	defer lw.mut.Unlock()
	lst := []appsv1.Deployment{}
	for _, depl := range lw.items {
		lst = append(lst, depl)
	}
	return &appsv1.DeploymentList{Items: lst}, nil
}

func (lw *fakeDeploymentListerWatcher) Watch(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
	return lw.watcher, nil
}

func (lw *fakeDeploymentListerWatcher) getWatcher() *closeableWatcher {
	return lw.watcher
}

// addDeployment adds d to the internal deployments list, or overwrites it if it
// already existed. in either case, it will be returned by a future call to List.
// in the former case, an ADD event if sent if sendEvent is true, and in the latter
// case, a MODIFY event is sent if sendEvent is true
func (lw *fakeDeploymentListerWatcher) addDeployment(d appsv1.Deployment, sendEvent bool) error {
	lw.mut.Lock()
	defer lw.mut.Unlock()
	_, existed := lw.items[d.ObjectMeta.Name]
	lw.items[d.ObjectMeta.Name] = d
	if sendEvent {
		if existed {
			return lw.watcher.Modify(&d)
		} else {
			return lw.watcher.Add(&d)
		}
	}
	return nil
}
