package main

import (
	"net/http/httptest"
	"testing"

	echo "github.com/labstack/echo/v4"
	"github.com/stretchr/testify/suite"
)

type InterceptorSuite struct {
	suite.Suite
}

func TestInterceptor(t *testing.T) {
	suite.Run(t, new(InterceptorSuite))
}

func newTestCtx(method, path string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	return e, e.NewContext(req, rec), rec
}
