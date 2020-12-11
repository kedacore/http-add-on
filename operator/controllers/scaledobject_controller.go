/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

// ScaledObjectReconciler reconciles a ScaledObject object
type ScaledObjectReconciler struct {
	K8sCl                 *kubernetes.Clientset
	K8sDynamicCl          dynamic.Interface
	ExternalScalerAddress string
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects/status,verbs=get;update;patch

func (r *ScaledObjectReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("scaledobject", req.NamespacedName)
	so := &httpv1alpha1.ScaledObject{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, so); err != nil {
		r.Log.Error(err, "Getting the HTTP Scaled obj")
		return ctrl.Result{}, err
	}

	appName := so.Spec.AppName
	image := so.Spec.Image
	port := so.Spec.Port

	r.Log.Info("App Name: %s, image: %s, port: %d", appName, image, port)

	appsCl := r.K8sCl.AppsV1().Deployments("cscaler")
	deployment := k8s.NewDeployment(req.Namespace, appName, image, port)
	// TODO: watch the deployment until it reaches ready state
	if _, err := appsCl.Create(deployment); err != nil {
		r.Log.Error(err, "Creating deployment")
		return ctrl.Result{}, err
	}

	coreCl := r.K8sCl.CoreV1().Services("cscaler")
	service := k8s.NewService(req.Namespace, appName, port)
	if _, err := coreCl.Create(service); err != nil {
		r.Log.Error(err, "Creating service")
		return ctrl.Result{}, err
	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's
	// deployment
	coreScaledObject := k8s.NewScaledObject(
		req.Namespace,
		req.Name,
		req.Name,
		r.ExternalScalerAddress,
	)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(r.K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(req.Namespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		r.Log.Error(err, "Creating scaledobject")
		return ctrl.Result{}, err
	}

	// TODO: install a dedicated interceptor deployment for this app
	// TODO: install a dedicated external scaler for this app

	return ctrl.Result{
		// TODO: should we requeue immediately?
		RequeueAfter: time.Millisecond * 200,
	}, nil
}

func (r *ScaledObjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.ScaledObject{}).
		Complete(r)
}
