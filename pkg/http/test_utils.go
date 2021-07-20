package http

import (
	nethttp "net/http"
	"net/http/httptest"
)

func NewTestCtx(
	method,
	path string,
) (*nethttp.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	return req, rec
}
