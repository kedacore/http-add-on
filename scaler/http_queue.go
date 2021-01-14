// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import "sync"

type httpQueue interface {
	pendingCounter() int
}

type reqCounter struct {
	count   int
	mut *sync.RWMutex
}

func (r *reqCounter) inc() {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.count++
}

func (r *reqCounter) dec() {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.count--
}

func (r *reqCounter) pendingCounter() int {
	r.mut.RLock()
	defer r.mut.RUnlock()
	return r.count
}
