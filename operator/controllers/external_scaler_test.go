package controllers

import (
	"time"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ExternalScaler", func() {
	Context("Creating the external scaler", func() {
		var testInfra *commonTestInfra
		BeforeEach(func() {
			testInfra = newCommonTestInfra("testns", "testapp")
		})
		It("Should properly create the Deployment and Service", func() {
			err := createExternalScaler(
				testInfra.ctx,
				testInfra.cfg,
				testInfra.cl,
				testInfra.logger,
				&testInfra.httpso,
			)
			Expect(err).To(BeNil())

			// // make sure that httpso has the right conditions on it
			Expect(len(testInfra.httpso.Status.Conditions)).To(Equal(1))
			cond1 := testInfra.httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.CreatedExternalScaler))

			// check that the external scaler deployment was created
			deployment := new(appsv1.Deployment)
			err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
				Name:      testInfra.cfg.ExternalScalerDeploymentName(),
				Namespace: testInfra.cfg.Namespace,
			}, deployment)
			Expect(err).To(BeNil())

			// check that the external scaler service was created
			service := new(corev1.Service)
			err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
				Name:      testInfra.cfg.ExternalScalerServiceName(),
				Namespace: testInfra.cfg.Namespace,
			}, service)
			Expect(err).To(BeNil())

		})
	})

})
