package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("UserApp", func() {
	Context("Creating a user app", func() {
		It("Should properly create a deployment and a service", func() {
			ctx := context.Background()
			cl := fake.NewClientBuilder().Build()
			cfg := config.AppInfo{
				Name:      "testapp",
				Port:      8081,
				Image:     "arschles/testimg",
				Namespace: "testns",
			}
			logger := logr.Discard()
			httpso := &v1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testns",
					Name:      "testapp",
				},
				Spec: v1alpha1.HTTPScaledObjectSpec{
					AppName: "testname",
					Image:   "arschles/testapp",
					Port:    8081,
				},
			}
			err := createUserApp(ctx, cfg, cl, logger, httpso)
			Expect(err).To(BeNil())
		})
	})
})
