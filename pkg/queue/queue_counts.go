package queue

// Count is a snapshot of the HTTP pending request concurrency
// and the raw monotonic request counter, as reported by an
// interceptor pod.
type Count struct {
	Concurrency  int   `json:"Concurrency"`
	RequestCount int64 `json:"RequestCount"`
}

// Counts is a snapshot of the HTTP pending request counts
// for each host.
type Counts map[string]Count
