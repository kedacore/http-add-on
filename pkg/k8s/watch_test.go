package k8s

import (
	"context"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// closeableWatcher is a watch.Interface that can be closed
// and optionally reopened
type closeableWatcher struct {
}

func newCloseableWatcher() *closeableWatcher {
	return &closeableWatcher{}
}

func (w *closeableWatcher) Stop() {
}

func (w *closeableWatcher) ResultChan() <-chan watch.Event {
	return nil
}

func (w *closeableWatcher) closeOpenChans(allowReopen bool) {
}

func (w *closeableWatcher) Add(d *appsv1.Deployment) {
}

func (w *closeableWatcher) Modify(d *appsv1.Deployment) {
}

type fakeDeploymentListerWatcher struct {
	mut         *sync.RWMutex
	fakeWatcher *closeableWatcher
	recorder    *watch.Recorder
	items       map[string]appsv1.Deployment
}

func newFakeDeploymentListerWatcher() *fakeDeploymentListerWatcher {
	w := newCloseableWatcher()
	r := watch.NewRecorder(w)
	return &fakeDeploymentListerWatcher{
		mut:         new(sync.RWMutex),
		fakeWatcher: w,
		recorder:    r,
		items:       map[string]appsv1.Deployment{},
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
	return lw.fakeWatcher, nil
}

func (lw *fakeDeploymentListerWatcher) getFakeWatcher() *closeableWatcher {
	return lw.fakeWatcher
}

func (lw *fakeDeploymentListerWatcher) getRecorder() *watch.Recorder {
	return lw.recorder
}

// addDeployment adds d to the internal deployments list, or overwrites it if it
// already existed. in either case, it will be returned by a future call to List.
// in the former case, an ADD event if sent if sendEvent is true, and in the latter
// case, a MODIFY event is sent if sendEvent is true
func (lw *fakeDeploymentListerWatcher) addDeployment(d appsv1.Deployment, sendEvent bool) {
	lw.mut.Lock()
	defer lw.mut.Unlock()
	_, existed := lw.items[d.ObjectMeta.Name]
	lw.items[d.ObjectMeta.Name] = d
	if sendEvent {
		if existed {
			lw.fakeWatcher.Modify(&d)
		} else {
			lw.fakeWatcher.Add(&d)
		}
	}
}
