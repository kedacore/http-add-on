package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInterceptor(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Interceptor Suite")
}
