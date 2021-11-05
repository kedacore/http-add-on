package queue

import (
	"encoding/json"
	"fmt"
)

// Counts is a snapshot of the HTTP pending request queue counts
// for each host.
// This is a json.Marshaler, json.Unmarshaler, and fmt.Stringer
// implementation.
//
// Use NewQueueCounts to create a new one of these.
type Counts struct {
	json.Marshaler
	json.Unmarshaler
	fmt.Stringer
	Counts map[string]int
}

// NewQueueCounts creates a new empty QueueCounts struct
func NewCounts() *Counts {
	return &Counts{
		Counts: map[string]int{},
	}
}

// Aggregate returns the total count across all hosts
func (q *Counts) Aggregate() int {
	agg := 0
	for _, count := range q.Counts {
		agg += count
	}
	return agg
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
