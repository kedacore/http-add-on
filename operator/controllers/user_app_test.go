package controllers

import (
	"github.com/kedacore/http-add-on/operator/controllers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("UserApp", func() {
	Context("Creating a user app", func() {
		It("Should properly create a deployment and a service", func() {
			cl := fake.NewSimpleClientset()
			cfg := config.AppInfo{}
			err := createUserApp(cfg, cl, logger, httpso)
			Expect(err).To(Equal(nil))
			actions := cl.Actions()
			Expect(len(actions)).To(Equal(2))
		})
	})
})
