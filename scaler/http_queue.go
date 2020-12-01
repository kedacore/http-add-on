package main

import "sync"

type httpQueue interface {
	pendingCounter() int
}

type reqCounter struct {
	i   int
	mut *sync.RWMutex
}

func (r *reqCounter) inc() {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.i++
}

func (r *reqCounter) dec() {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.i--
}

func (r *reqCounter) pendingCounter() int {
	r.mut.RLock()
	defer r.mut.RUnlock()
	return r.i
}
