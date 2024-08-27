package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServeContext(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("foo", "bar")
		_, err := w.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("error writing message to client from handler")
		}
	})
	addr := "localhost:1234"
	const cancelDur = 500 * time.Millisecond
	go func() {
		time.Sleep(cancelDur)
		done()
	}()
	start := time.Now()
	err := ServeContext(ctx, addr, hdl, nil)
	elapsed := time.Since(start)

	r.Error(err)
	r.True(errors.Is(err, http.ErrServerClosed), "error is not a http.ErrServerClosed (%w)", err)
	r.Greater(elapsed, cancelDur)
	r.Less(elapsed, cancelDur*4)
}

func TestServeContextWithTLS(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	hdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("foo", "bar")
		_, err := w.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("error writing message to client from handler")
		}
	})
	addr := "localhost:1234"
	const cancelDur = 500 * time.Millisecond
	go func() {
		time.Sleep(cancelDur)
		done()
	}()
	start := time.Now()
	tlsConfig := tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair("../../certs/tls.crt", "../../certs/tls.key")
			return &cert, err
		},
	}
	err := ServeContext(ctx, addr, hdl, &tlsConfig)
	elapsed := time.Since(start)

	r.Error(err)
	r.True(errors.Is(err, http.ErrServerClosed), "error is not a http.ErrServerClosed (%w)", err)
	r.Greater(elapsed, cancelDur)
	r.Less(elapsed, cancelDur*4)
}
