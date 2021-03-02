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

type fakeQueueCounter struct {
	resizedCh chan int
}

func (f *fakeQueueCounter) Resize(i int) error {
	f.resizedCh <- i
	return nil
}

func (f *fakeQueueCounter) Current() (int, error) {
	return 0, nil
}

type fakeQueueCountReader struct {
	current int
	err     error
}

func (f *fakeQueueCountReader) Current() (int, error) {
	return f.current, f.err
}
