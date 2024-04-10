package queue

import (
	"encoding/json"
	"fmt"
)

// Count is a snapshot of the HTTP pending request (Concurrency)
// count and RPS

type Count struct {
	Concurrency int
	RPS         float64
}

// Count is a snapshot of the HTTP pending request (Concurrency)
// count and RPS for each host.
// This is a json.Marshaler, json.Unmarshaler, and fmt.Stringer
// implementation.
//
// Use NewQueueCounts to create a new one of these.
type Counts struct {
	json.Marshaler
	json.Unmarshaler
	fmt.Stringer
	Counts map[string]Count
}

// NewQueueCounts creates a new empty QueueCounts struct
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
