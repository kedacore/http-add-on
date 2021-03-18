package k8s

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	apiappsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

// DeploymentFastGetter gets a Deployment quickly and without making a request
// directly to the Kubernetes API on each call to Get.
// It is backed by a single process that periodically updates the data in the background
type DeploymentFastGetter interface {
	Get(name string) (*apiappsv1.Deployment, error)
}

type k8sAPIBackgroundDeploymentGetter struct {
	rwm    *sync.RWMutex
	cached *apiappsv1.Deployment
}

func NewK8sAPIDeploymentFastGetter(
	ctx context.Context,
	name string,
	updateDur time.Duration,
	cl typedappsv1.DeploymentInterface,
) (DeploymentFastGetter, error) {
	const op = "NewK8sAPIDeploymentFastGetter"
	depl, err := cl.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, op)
	}
	ret := &k8sAPIBackgroundDeploymentGetter{
		rwm:    new(sync.RWMutex),
		cached: depl,
	}

	go func() {
		ticker := time.NewTicker(updateDur)
		defer ticker.Stop()
		for range ticker.C {
			depl, err := cl.Get(ctx, name, metav1.GetOptions{})

			if err != nil {
				// TODO: handle error cases - what happens if it's gotten too stale
				continue
			}
			ret.rwm.Lock()
			ret.cached = depl
			ret.rwm.Unlock()
		}
	}()

	return ret, nil
}

func (k *k8sAPIBackgroundDeploymentGetter) Get(name string) (*apiappsv1.Deployment, error) {
	k.rwm.RLock()
	defer k.rwm.RUnlock()
	return k.cached, nil
}
