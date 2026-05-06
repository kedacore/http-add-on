// /*

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package http

import (
	"context"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

type commonTestInfra struct {
	ns      string
	appName string
	ctx     context.Context
	cl      client.Client
	logger  logr.Logger
	httpso  httpv1alpha1.HTTPScaledObject
}

func newCommonTestInfra(namespace, appName string) *commonTestInfra {
	localScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(localScheme))
	utilruntime.Must(httpv1alpha1.AddToScheme(localScheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(localScheme))

	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(localScheme).Build()
	logger := logr.Discard()

	httpso := httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      appName,
			Labels: map[string]string{
				"label": "a",
			},
			Annotations: map[string]string{
				"annotation": "b",
			},
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Service: appName,
				Port:    8081,
			},
			Hosts: []string{"myhost1.com", "myhost2.com"},
		},
	}

	return &commonTestInfra{
		ns:      namespace,
		appName: appName,
		ctx:     ctx,
		cl:      cl,
		logger:  logger,
		httpso:  httpso,
	}
}

func newCommonTestInfraWithSkipScaledObjectCreation(namespace, appName string) *commonTestInfra {
	localScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(localScheme))
	utilruntime.Must(httpv1alpha1.AddToScheme(localScheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(localScheme))

	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(localScheme).Build()
	logger := logr.Discard()

	httpso := httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      appName,
			Annotations: map[string]string{
				"httpscaledobject.keda.sh/skip-scaledobject-creation": "true",
			},
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Name:    appName,
				Service: appName,
				Port:    8081,
			},
			Hosts: []string{"myhost1.com", "myhost2.com"},
		},
	}

	return &commonTestInfra{
		ns:      namespace,
		appName: appName,
		ctx:     ctx,
		cl:      cl,
		logger:  logger,
		httpso:  httpso,
	}
}
