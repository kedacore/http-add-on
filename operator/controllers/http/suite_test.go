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
	"testing"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// var cfg *rest.Config
// var k8sClient client.Client
// var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	// testEnv = &envtest.Environment{
	// 	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
	// 	ErrorIfCRDPathMissing: true,
	// }

	var err error
	// cfg is defined in this file globally.
	// cfg, err = testEnv.Start()
	// Expect(err).NotTo(HaveOccurred())
	// Expect(cfg).NotTo(BeNil())

	err = httpv1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kedav1alpha1.AddToScheme(clientgoscheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	// k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	// Expect(err).NotTo(HaveOccurred())
	// Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	// err := testEnv.Stop()
	// Expect(err).NotTo(HaveOccurred())
})

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
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: &httpv1alpha1.ScaleTargetRef{
				Deployment: appName,
				Service:    appName,
				Port:       8081,
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
