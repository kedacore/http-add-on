package handler

import (
	"net/http/httputil"
	"sync"
)

// bufferSize is the size of buffers used for copying HTTP request/response bodies.
// This matches the buffer size used in Go's standard library net/http/httputil.ReverseProxy:
// https://github.com/golang/go/blob/go1.25.5/src/net/http/httputil/reverseproxy.go#L657
const bufferSize = 32 * 1024

// bufPool recycles buffers used by the reverse proxy to reduce memory allocations.
type bufPool struct {
	pool *sync.Pool
}

func newBufferPool() httputil.BufferPool {
	return &bufPool{
		pool: &sync.Pool{
			New: func() any {
				b := make([]byte, bufferSize)
				return &b
			},
		},
	}
}

func (bp *bufPool) Get() []byte {
	return *(bp.pool.Get().(*[]byte))
}

func (bp *bufPool) Put(b []byte) {
	bp.pool.Put(&b)
}
