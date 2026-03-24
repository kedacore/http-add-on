package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	combinedLogFormat     = `%s - - [%s] "%s %s %s" %d %d "%s" "%s"`
	combinedLogTimeFormat = "02/Jan/2006:15:04:05 -0700"
)

type Logging struct {
	logger logr.Logger
	next   http.Handler
}

func NewLogging(next http.Handler, logger logr.Logger) *Logging {
	return &Logging{
		logger: logger,
		next:   next,
	}
}

var _ http.Handler = (*Logging)(nil)

func (lm *Logging) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLogger(r, lm.logger.WithName("LoggingMiddleware"))
	rw := newInstrumentedResponseWriter(w)

	startTime := time.Now()
	defer lm.logAsync(rw, r, startTime)

	lm.next.ServeHTTP(rw, r)
}

func (lm *Logging) logAsync(rw *instrumentedResponseWriter, r *http.Request, startTime time.Time) {
	go lm.log(rw, r, startTime)
}

func (lm *Logging) log(rw *instrumentedResponseWriter, r *http.Request, startTime time.Time) {
	ctx := r.Context()
	logger := util.LoggerFromContext(ctx)

	timestamp := startTime.Format(combinedLogTimeFormat)
	log := fmt.Sprintf(
		combinedLogFormat,
		r.RemoteAddr,
		timestamp,
		r.Method,
		r.URL.Path,
		r.Proto,
		rw.statusCode,
		rw.bytesWritten,
		r.Referer(),
		r.UserAgent(),
	)
	logger.Info(log)
}
