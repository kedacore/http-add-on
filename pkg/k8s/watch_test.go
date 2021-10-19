package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// reopeningWatcher is a watch.Interface that, when closed, is
// reopened immediately upon the next call to ResultChan().
//
// it's similar to the RaceFreeFakeWatcher in k8s.io/apimachinery/pkg/watch
// except that requires you to manually call a Reset() function to
// reopen the watcher. The automatic reopen functionality is
// necessary.
type reopeningWatcher struct {
	mut    *sync.RWMutex
	ch     chan watch.Event
	events []watch.Event
	closed bool
}

func newReopeningWatcher() *reopeningWatcher {
	return &reopeningWatcher{
		mut:    new(sync.RWMutex),
		ch:     make(chan watch.Event),
		events: nil,
		closed: false,
	}
}

func (w *reopeningWatcher) Stop() {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if !w.closed {
		close(w.ch)
		w.closed = true
	}
}

func (w *reopeningWatcher) ResultChan() <-chan watch.Event {
	w.mut.Lock()
	defer w.mut.Unlock()
	if w.closed {
		w.ch = make(chan watch.Event)
		w.closed = false
	}
	return w.ch
}

func (w *reopeningWatcher) Add(d *appsv1.Deployment, dur time.Duration) error {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if w.closed {
		return errors.New("watcher is closed")
	}
	evt := watch.Event{
		Type:   watch.Added,
		Object: d,
	}
	select {
	case w.ch <- evt:
	case <-time.After(dur):
		return fmt.Errorf("couldn't send ADD event within %s", dur)
	}
	w.events = append(w.events, evt)
	return nil
}

func (w *reopeningWatcher) Modify(d *appsv1.Deployment, dur time.Duration) error {
	w.mut.RLock()
	defer w.mut.RUnlock()
	if w.closed {
		return errors.New("watcher is closed")
	}
	evt := watch.Event{
		Type:   watch.Modified,
		Object: d,
	}
	select {
	case w.ch <- evt:
	case <-time.After(dur):
		return fmt.Errorf("couldn't send MODIFY event within %s", dur)
	}
	w.events = append(w.events, evt)
	return nil
}

func (w *reopeningWatcher) getEvents() []watch.Event {
	w.mut.RLock()
	defer w.mut.RUnlock()
	return w.events
}

type fakeDeploymentListerWatcher struct {
	mut     *sync.RWMutex
	row     *reopeningWatcher
	items   map[string]appsv1.Deployment
	watchCB func()
}

var _ DeploymentListerWatcher = &fakeDeploymentListerWatcher{}

func newFakeDeploymentListerWatcher() *fakeDeploymentListerWatcher {
	return &fakeDeploymentListerWatcher{
		mut:   new(sync.RWMutex),
		row:   newReopeningWatcher(),
		items: map[string]appsv1.Deployment{},
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
	if lw.watchCB != nil {
		lw.watchCB()
	}
	return lw.row, nil
}

// addDeployment adds d to the internal deployments list, or overwrites it if it
// already existed. in either case, it will be returned by a future call to List.
// in the former case, an ADD event if sent if sendEvent is true, and in the latter
// case, a MODIFY event is sent if sendEvent is true
func (lw *fakeDeploymentListerWatcher) addDeployment(d appsv1.Deployment) {
	lw.mut.Lock()
	defer lw.mut.Unlock()
	lw.items[d.ObjectMeta.Name] = d
}

func (lw *fakeDeploymentListerWatcher) addDeploymentAndSendEvent(d appsv1.Deployment, waitDur time.Duration) error {
	lw.mut.RLock()
	defer lw.mut.RUnlock()
	_, existed := lw.items[d.GetName()]
	lw.items[d.GetName()] = d
	if existed {
		return lw.row.Modify(&d, waitDur)
	} else {
		return lw.row.Add(&d, waitDur)
	}

}
