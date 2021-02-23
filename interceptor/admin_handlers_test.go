package main

import (
	"net/http/httptest"

	echo "github.com/labstack/echo/v4"
)

type fakeQueueCountReader struct {
	current int
	err     error
}

func (f *fakeQueueCountReader) Current() (int, error) {
	return f.current, f.err
}

func (i *InterceptorSuite) TestQueueSizeHandler() {
	reader := &fakeQueueCountReader{
		current: 123,
		err:     nil,
	}

	handler := newQueueSizeHandler(reader)
	req := httptest.NewRequest("GET", "/queue", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	echoCtx := e.NewContext(req, rec)
	err := handler(echoCtx)
	i.NoError(err)
}
