package main

type ContextKey int

const (
	ContextKeyLogger ContextKey = iota
	ContextKeyHTTPSO
)
