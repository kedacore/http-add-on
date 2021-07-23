package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	pkghttp "github.com/kedacore/http-add-on/pkg/http"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
)

func TestQueueSizeHandlerSuccess(t *testing.T) {
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		current: 123,
		err:     nil,
	}

	handler := newSizeHandler(lggr, reader)
	req, rec := pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(200, rec.Code, "response code")
	respMap := map[string]int{}
	decodeErr := json.NewDecoder(rec.Body).Decode(&respMap)
	r.NoError(decodeErr)
	r.Equalf(1, len(respMap), "response JSON length was not 1")
	sizeVal, ok := respMap["sample.com"]
	r.Truef(ok, "'sample.com' entry not available in return JSON")
	r.Equalf(reader.current, sizeVal, "returned JSON queue size was wrong")

	reader.err = errors.New("test error")
	req, rec = pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(500, rec.Code, "response code was not expected")
}

func TestQueueSizeHandlerFail(t *testing.T) {
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		current: 0,
		err:     errors.New("test error"),
	}

	handler := newSizeHandler(lggr, reader)
	req, rec := pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(500, rec.Code, "response code")
}

func TestQueueSizeHandlerIntegration(t *testing.T) {
	ctx := context.Background()
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		current: 50,
		err:     nil,
	}

	hdl := kedanet.NewTestHTTPHandlerWrapper(newSizeHandler(lggr, reader))
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	httpCl := srv.Client()
	counts, err := GetCounts(ctx, lggr, httpCl, *url)
	r.NoError(err)
	r.Equal(1, len(counts.Counts))
	for _, val := range counts.Counts {
		r.Equal(reader.current, val)
	}
	reqs := hdl.IncomingRequests()
	r.Equal(1, len(reqs))

}
