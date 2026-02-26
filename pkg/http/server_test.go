package http

import (
	"context"
	"crypto/tls"
	"fmt"
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
		rc := http.NewResponseController(w)
		if err := rc.EnableFullDuplex(); err != nil {
			t.Fatalf("error enabling full duplex: %v", err)
		}

		w.Header().Set("foo", "bar")
		_, err := w.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("error writing message to client from handler")
		}
	})
	addr := "localhost:1234"
	const waitDur = 100 * time.Millisecond
	const cancelDur = 400 * time.Millisecond
	go func() {
		time.Sleep(waitDur)

		// send a request so the handler runs
		resp, err := http.Get("http://" + addr)
		if err != nil {
			panic(fmt.Sprintf("error sending request to the server: %v", err))
		}
		defer resp.Body.Close()

		time.Sleep(cancelDur)
		done()
	}()
	start := time.Now()
	err := ServeContext(ctx, addr, hdl, nil)
	elapsed := time.Since(start)

	r.Error(err)
	r.ErrorIs(err, http.ErrServerClosed, "error is not a http.ErrServerClosed (%w)", err)
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
	r.ErrorIs(err, http.ErrServerClosed, "error is not a http.ErrServerClosed (%w)", err)
	r.Greater(elapsed, cancelDur)
	r.Less(elapsed, cancelDur*4)
}
