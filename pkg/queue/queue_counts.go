package queue

import (
	"encoding/json"
	"fmt"
)

// Count is a snapshot of the HTTP pending request (Concurrency),
// the raw monotonic request counter (RequestCount), and the
// computed request rate (RPS). The interceptor populates
// Concurrency and RequestCount; the scaler computes RPS.
type Count struct {
	Concurrency  int
	RequestCount int64
	// TODO(v1): the naming "RPS" is incorrect, it is rather "Rate" or "RequestRate"
	RPS float64
}

// Counts is a snapshot of the HTTP pending request counts
// for each host.
// This is a json.Marshaler, json.Unmarshaler, and fmt.Stringer
// implementation.
//
// Use NewCounts to create a new one of these.
type Counts struct {
	json.Marshaler
	json.Unmarshaler
	fmt.Stringer
	Counts map[string]Count
}

// NewCounts creates a new empty Counts struct
func NewCounts() *Counts {
	return &Counts{
		Counts: map[string]Count{},
	}
}

// Aggregate returns the total count across all hosts
func (q *Counts) Aggregate() Count {
	res := Count{}
	for _, count := range q.Counts {
		res.Concurrency += count.Concurrency
		res.RequestCount += count.RequestCount
		res.RPS += count.RPS
	}
	return res
}

// MarshalJSON implements json.Marshaler
func (q *Counts) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.Counts)
}

// UnmarshalJSON implements json.Unmarshaler
func (q *Counts) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &q.Counts)
}

// String implements fmt.Stringer
func (q *Counts) String() string {
	return fmt.Sprintf("%v", q.Counts)
}
