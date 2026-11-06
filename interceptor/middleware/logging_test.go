package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"

	"github.com/kedacore/http-add-on/pkg/util"
)

func TestLoggingLog(t *testing.T) {
	var logged string
	logger := funcr.New(func(_, args string) { logged = args }, funcr.Options{})

	lm := NewLogging(nil, logger)
	r := httptest.NewRequest("GET", "/a%0Afake", nil)
	r = util.RequestWithLogger(r, logger)

	lm.log(newInstrumentedResponseWriter(httptest.NewRecorder()), r, time.Now())

	if want := `GET /a%0Afake HTTP/1.1`; !strings.Contains(logged, want) {
		t.Errorf("log line %s does not contain %q", logged, want)
	}
}
