package main

import (
	"net/http/httptest"

	"github.com/kedacore/http-add-on/pkg/http"
	echo "github.com/labstack/echo/v4"
)

func newTestCtx(method, path string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	return e, e.NewContext(req, rec), rec
}

var _ http.QueueCounter = &fakeQueueCounter{}

type fakeQueueCounter struct {
	resizedCh chan int
}

func (f *fakeQueueCounter) Resize(host string, i int) error {
	f.resizedCh <- i
	return nil
}

func (f *fakeQueueCounter) Current() (*http.QueueCounts, error) {
	ret := http.NewQueueCounts()
	ret.Counts = map[string]int{
		"sample.com": 0,
	}
	return ret, nil
}

var _ http.QueueCountReader = &fakeQueueCountReader{}

type fakeQueueCountReader struct {
	current int
	err     error
}

func (f *fakeQueueCountReader) Current() (*http.QueueCounts, error) {
	ret := http.NewQueueCounts()
	ret.Counts = map[string]int{
		"sample.com": f.current,
	}
	return ret, f.err
}
