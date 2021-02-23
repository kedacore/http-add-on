package main

import (
	"encoding/json"
	"errors"
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

func (i *InterceptorSuite) TestQueueSizeHandlerSuccess() {
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
	i.Equal(200, rec.Code, "response code")
	respMap := map[string]int{}
	decodeErr := json.NewDecoder(rec.Body).Decode(&respMap)
	i.NoError(decodeErr)
	i.Equalf(1, len(respMap), "response JSON length was not 1")
	sizeVal, ok := respMap["current_size"]
	i.Truef(ok, "'current_size' entry not available in return JSON")
	i.Equalf(reader.current, sizeVal, "returned JSON queue size was wrong")
	reader.err = errors.New("test error")
	i.Error(handler(echoCtx))
}

func (i *InterceptorSuite) TestQueueSizeHandlerFail() {
	reader := &fakeQueueCountReader{
		current: 0,
		err:     errors.New("test error"),
	}

	handler := newQueueSizeHandler(reader)
	req := httptest.NewRequest("GET", "/queue", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	echoCtx := e.NewContext(req, rec)
	err := handler(echoCtx)
	i.Error(err)
	i.Equal(500, rec.Code, "response code")
}
