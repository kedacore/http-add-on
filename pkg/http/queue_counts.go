package http

import (
	"encoding/json"
	"fmt"
)

// QueueCounts is a snapshot of the HTTP pending request queue counts
// for each host. This is a json.Marshaler and json.Unmarshaler implementation.
//
// Use NewQueueCounts to create a new one of these.
type QueueCounts struct {
	json.Marshaler
	json.Unmarshaler
	fmt.Stringer
	Counts map[string]int
}

// NewQueueCounts creates a new empty QueueCounts struct
func NewQueueCounts() *QueueCounts {
	return &QueueCounts{
		Counts: map[string]int{},
	}
}

// MarshalJSON implements json.Marshaler
func (q *QueueCounts) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.Counts)
}

// UnmarshalJSON implements json.Unmarshaler
func (q *QueueCounts) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &q.Counts)
}

// String implements fmt.Stringer
func (q *QueueCounts) String() string {
	return fmt.Sprintf("%v", q.Counts)
}
