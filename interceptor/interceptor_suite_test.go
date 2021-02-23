package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type InterceptorSuite struct {
	suite.Suite
}

func TestInterceptor(t *testing.T) {
	suite.Run(t, new(InterceptorSuite))
}
