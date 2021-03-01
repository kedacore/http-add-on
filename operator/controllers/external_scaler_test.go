package controllers

import (
	"context"
	"time"

	logrtest "github.com/go-logr/logr/testing"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ExternalScaler", func() {
	Context("Creating the external scaler", func() {
		It("Should properly create the Deployment and Service", func() {
			const name = "testapp"
			const namespace = "testns"
			ctx := context.Background()
			cl := fake.NewFakeClient()
			cfg := config.AppInfo{
				App: config.App{
					Name:      name,
					Port:      8081,
					Image:     "arschles/testimg",
					Namespace: namespace,
				},
			}
			logger := logrtest.NullLogger{}
			httpso := &v1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: v1alpha1.HTTPScaledObjectSpec{
					AppName: name,
					Image:   "arschles/testapp",
					Port:    8081,
				},
			}
			err := createExternalScaler(ctx, cfg, cl, logger, httpso)
			Expect(err).To(BeNil())

			// // make sure that httpso has the right conditions on it
			Expect(len(httpso.Status.Conditions)).To(Equal(1))
			cond1 := httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.CreatedExternalScaler))

			// check that the external scaler deployment was created
			deployment := new(appsv1.Deployment)
			err = cl.Get(ctx, client.ObjectKey{
				Name:      cfg.ExternalScalerDeploymentName(),
				Namespace: cfg.App.Namespace,
			}, deployment)
			Expect(err).To(BeNil())

			// check that the external scaler service was created
			service := new(corev1.Service)
			err = cl.Get(ctx, client.ObjectKey{
				Name:      cfg.ExternalScalerServiceName(),
				Namespace: cfg.App.Namespace,
			}, service)
			Expect(err).To(BeNil())

		})
	})

})
