package queue

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		concurrency: 123,
		err:         nil,
	}

	handler := newSizeHandler(lggr, reader)
	req, rec := pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(200, rec.Code, "response code")
	respMap := Counts{}
	decodeErr := json.NewDecoder(rec.Body).Decode(&respMap)
	r.NoError(decodeErr)
	r.Lenf(respMap, 1, "response JSON length was not 1")
	sizeVal, ok := respMap["sample.com"]
	r.Truef(ok, "'sample.com' entry not available in return JSON")
	r.Equalf(reader.concurrency, sizeVal.Concurrency, "returned JSON concurrent size was wrong")

	reader.err = errors.New("test error")
	req, rec = pkghttp.NewTestCtx("GET", "/queue")
	handler.ServeHTTP(rec, req)
	r.Equal(500, rec.Code, "response code was not expected")
}

func TestQueueSizeHandlerFail(t *testing.T) {
	lggr := logr.Discard()
	r := require.New(t)
	reader := &FakeCountReader{
		concurrency: 0,
		err:         errors.New("test error"),
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
		concurrency: 50,
		err:         nil,
	}

	hdl := kedanet.NewTestHTTPHandlerWrapper(newSizeHandler(lggr, reader))
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	httpCl := srv.Client()
	counts, err := GetCounts(t.Context(), httpCl, *url)
	r.NoError(err)
	r.Len(counts, 1)
	for _, val := range counts {
		r.Equal(reader.concurrency, val.Concurrency)
	}
	reqs := hdl.IncomingRequests()
	r.Len(reqs, 1)
}

// TestGetCountsUnresponsiveInterceptor pins down the contract the scaler relies
// on: an interceptor that accepts the connection but never answers must fail
// the request rather than block the caller forever.
func TestGetCountsUnresponsiveInterceptor(t *testing.T) {
	r := require.New(t)
	const timeout = 100 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	r.NoError(err)

	start := time.Now()
	_, err = GetCounts(t.Context(), &http.Client{Timeout: timeout}, *srvURL)
	r.Error(err)
	r.Less(time.Since(start), 10*timeout, "the request must be bounded by the client timeout")
}

func TestGetCountsCancelledContext(t *testing.T) {
	r := require.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	r.NoError(err)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err = GetCounts(ctx, srv.Client(), *srvURL)
	r.ErrorIs(err, context.Canceled)
}

func TestGetCountsNonOKStatus(t *testing.T) {
	r := require.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "error getting queue size", http.StatusInternalServerError)
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	r.NoError(err)

	_, err = GetCounts(t.Context(), srv.Client(), *srvURL)
	r.Error(err)
	r.Contains(err.Error(), "unexpected status")
}
