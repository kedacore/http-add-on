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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

type commonTestInfra struct {
	ns      string
	appName string
	ctx     context.Context
	cl      client.Client
	logger  logr.Logger
	httpso  v1alpha1.HTTPScaledObject
}

func newCommonTestInfra(namespace, appName string) *commonTestInfra {
	ctx := context.Background()
	cl := fake.NewFakeClient()
	logger := logr.Discard()

	httpso := v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      appName,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: &v1alpha1.ScaleTargetRef{
				Deployment: appName,
				Service:    appName,
				Port:       8081,
			},
			Replicas: v1alpha1.ReplicaStruct{
				Min: 0,
				Max: 20,
			},
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
