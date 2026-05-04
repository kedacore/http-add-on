package queue

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func FuzzCountsJSON(f *testing.F) {
	f.Add([]byte(`{"host1":{"Concurrency":1,"RequestCount":10}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"":{"Concurrency":0,"RequestCount":0}}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var c1 Counts
		if err := json.Unmarshal(data, &c1); err != nil {
			return
		}

		encoded, err := json.Marshal(c1)
		if err != nil {
			t.Fatalf("Marshal failed after successful Unmarshal: %v", err)
		}

		var c2 Counts
		if err := json.Unmarshal(encoded, &c2); err != nil {
			t.Fatalf("round-trip Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(c1, c2) {
			t.Errorf("round-trip mismatch:\n  original: %v\n  decoded:  %v", c1, c2)
		}
	})
}

func FuzzRequestsBuckets(f *testing.F) {
	// Each record is 6 bytes: 4 bytes for time offset (seconds), 2 bytes for value.
	f.Add([]byte{0, 0, 0, 0, 0, 10})
	f.Add([]byte{0, 0, 0, 5, 0, 20, 0, 0, 0, 10, 0, 30})
	f.Add([]byte{0, 0, 0, 0, 0, 0})

	const (
		window      = 60 * time.Second
		granularity = 1 * time.Second
		recordSize  = 6
	)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	f.Fuzz(func(t *testing.T, data []byte) {
		buckets := NewRequestsBuckets(window, granularity)

		var lastTime time.Time
		for len(data) >= recordSize {
			offsetSec := binary.BigEndian.Uint32(data[:4])
			value := int(binary.BigEndian.Uint16(data[4:6]))
			data = data[recordSize:]

			// Cap the offset to avoid extreme time values.
			offsetSec %= (365 * 24 * 3600)

			now := base.Add(time.Duration(offsetSec) * time.Second)
			buckets.Record(now, value)
			lastTime = now
		}

		if lastTime.IsZero() {
			return
		}

		avg := buckets.WindowAverage(lastTime)
		if math.IsNaN(avg) || math.IsInf(avg, 0) {
			t.Errorf("WindowAverage returned %v", avg)
		}
		if avg < 0 {
			t.Errorf("WindowAverage returned negative value: %v", avg)
		}
	})
}
