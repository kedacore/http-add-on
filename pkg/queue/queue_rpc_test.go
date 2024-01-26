package queue

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	pkghttp "github.com/kedacore/http-add-on/pkg/http"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
)

func TestQueueSizeHandlerSuccess(t *testing.T) {
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		current:  123,
		activity: time.Now(),
		err:      nil,
	}

	handler := newSizeHandler(lggr, reader)
	req, rec := pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(200, rec.Code, "response code")
	respMap := map[string]Count{}
	decodeErr := json.NewDecoder(rec.Body).Decode(&respMap)
	r.NoError(decodeErr)
	r.Equalf(1, len(respMap), "response JSON length was not 1")
	respItem, ok := respMap["sample.com"]
	r.Truef(ok, "'sample.com' entry not available in return JSON")
	r.Equalf(reader.current, respItem.Requests, "returned JSON queue size was wrong")
	r.WithinDuration(reader.activity, respItem.Activity, 200*time.Millisecond, "returned JSON activity was wrong")

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
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		current:  50,
		activity: time.Now(),
		err:      nil,
	}

	hdl := kedanet.NewTestHTTPHandlerWrapper(newSizeHandler(lggr, reader))
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	httpCl := srv.Client()
	counts, err := GetCounts(httpCl, *url)
	r.NoError(err)
	r.Equal(1, len(counts.Counts))
	for _, val := range counts.Counts {
		r.Equal(reader.current, val.Requests)
		r.WithinDuration(reader.activity, val.Activity, 200*time.Millisecond)
	}
	reqs := hdl.IncomingRequests()
	r.Equal(1, len(reqs))
}
