package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	CombinedLogFormat     = `%s %s %s [%s] "%s %s %s" %d %d "%s" "%s"`
	CombinedLogTimeFormat = "02/Jan/2006:15:04:05 -0700"
	CombinedLogBlankValue = "-"
)

type LoggingMiddleware struct {
	logger          logr.Logger
	upstreamHandler http.Handler
}

func NewLoggingMiddleware(logger logr.Logger, upstreamHandler http.Handler) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger:          logger,
		upstreamHandler: upstreamHandler,
	}
}

var _ http.Handler = (*LoggingMiddleware)(nil)

func (lm *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var sw Stopwatch

	ctx := r.Context()

	ctx = context.WithValue(ctx, ContextKeyLogger, lm.logger)
	r = r.WithContext(ctx)

	w = NewLoggingResponseWriter(w)

	defer lm.logAsync(w, r, &sw)()

	sw.Start()
	defer sw.Stop()

	lm.upstreamHandler.ServeHTTP(w, r)
}

func (lm *LoggingMiddleware) logAsync(w http.ResponseWriter, r *http.Request, sw *Stopwatch) func() {
	signaler := util.NewSignaler()

	go lm.log(w, r, sw, signaler)

	return func() {
		go signaler.Signal()
	}
}

func (lm *LoggingMiddleware) log(w http.ResponseWriter, r *http.Request, sw *Stopwatch, signaler util.Signaler) {
	ctx := r.Context()

	logger, _ := ctx.Value(ContextKeyLogger).(logr.Logger)
	logger = logger.WithName("LoggingMiddleware")

	lrw := w.(*LoggingResponseWriter)

	if err := signaler.Wait(ctx); err != nil && err != context.Canceled {
		logger.Error(err, "failed to wait signal")
	}

	timestamp := sw.StartTime().Format(CombinedLogTimeFormat)
	log := fmt.Sprintf(
		CombinedLogFormat,
		r.RemoteAddr,
		CombinedLogBlankValue,
		CombinedLogBlankValue,
		timestamp,
		r.Method,
		r.URL.Path,
		r.Proto,
		lrw.StatusCode(),
		lrw.BytesWritten(),
		r.Referer(),
		r.UserAgent(),
	)
	logger.Info(log)
}
