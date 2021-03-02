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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("UserApp", func() {
	Context("Creating a user app", func() {
		It("Should properly create a deployment and a service", func() {
			ctx := context.Background()
			cl := fake.NewFakeClient()
			cfg := config.AppInfo{
				Name:      "testapp",
				Port:      8081,
				Image:     "arschles/testimg",
				Namespace: "testns",
			}
			logger := logrtest.NullLogger{}
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
			// make sure that httpso has the right conditions on it
			Expect(len(httpso.Status.Conditions)).To(Equal(2))

			cond1 := httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.AppDeploymentCreated))

			cond2 := httpso.Status.Conditions[1]
			cond2ts, err := time.Parse(time.RFC3339, cond2.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond2ts) >= 0).To(BeTrue())
			Expect(cond2.Type).To(Equal(v1alpha1.Created))
			Expect(cond2.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond2.Reason).To(Equal(v1alpha1.AppServiceCreated))

			// check the deployment that was created
			deployment := &appsv1.Deployment{}
			err = cl.Get(ctx, client.ObjectKey{
				Name:      cfg.Name,
				Namespace: cfg.Namespace,
			}, deployment)
			Expect(err).To(BeNil())
			Expect(deployment.Name).To(Equal(cfg.Name))
			Expect(len(deployment.Spec.Template.Spec.Containers)).To(Equal(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(cfg.Image))

			// check the service that was created
			svc := &corev1.Service{}
			err = cl.Get(ctx, client.ObjectKey{
				Name:      cfg.Name,
				Namespace: cfg.Namespace,
			}, svc)
			Expect(err).To(BeNil())
			Expect(svc.Name).To(Equal(cfg.Name))
			Expect(len(svc.Spec.Ports)).To(Equal(1))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].TargetPort.IntVal).To(Equal(cfg.Port))
			Expect(svc.Spec.Ports[0].TargetPort.Type).To(Equal(intstr.Int))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
		})
	})
})
