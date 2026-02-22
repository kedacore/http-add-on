// Based on https://github.com/knative/serving/blob/main/pkg/autoscaler/aggregation/bucketing.go

package queue

import (
	"math"
	"sync"
	"time"
)

const precision = 3

// RequestsBuckets keeps buckets that have been collected at a certain time.
type RequestsBuckets struct {
	bucketsMutex sync.RWMutex
	// buckets is a ring buffer indexed by timeToIndex() % len(buckets).
	// Each element represents a certain granularity of time, and the total
	// represented duration adds up to a window length of time.
	buckets []int

	// firstWrite holds the time when the first write has been made.
	// This time is reset to `now` when the very first write happens,
	// or when a first write happens after `window` time of inactivity.
	// The difference between `now` and `firstWrite` is used to compute
	// the number of eligible buckets for computation of average values.
	firstWrite time.Time

	// lastWrite stores the time when the last write was made.
	// This is used to detect when we have gaps in the data (i.e. more than a
	// granularity has expired since the last write) so that we can zero those
	// entries in the buckets array. It is also used when calculating the
	// WindowAverage to know how much of the buckets array represents valid data.
	lastWrite time.Time

	// granularity is the duration represented by each bucket in the buckets ring buffer.
	granularity time.Duration
	// window is the total time represented by the buckets ring buffer.
	window time.Duration
	// The total sum of all buckets within the window. This total includes
	// invalid buckets, e.g. buckets written to before firstTime or after
	// lastTime are included in this total.
	windowTotal int
}

// NewRequestsBuckets generates a new RequestsBuckets with the given
// granularity.
func NewRequestsBuckets(window, granularity time.Duration) *RequestsBuckets {
	// Number of buckets is `window` divided by `granularity`, rounded up.
	// e.g. 60s / 2s = 30.
	nb := math.Ceil(float64(window) / float64(granularity))
	return &RequestsBuckets{
		buckets:     make([]int, int(nb)),
		granularity: granularity,
		window:      window,
	}
}

// IsEmpty returns true if no data has been recorded for the `window` period.
func (t *RequestsBuckets) IsEmpty(now time.Time) bool {
	now = now.Truncate(t.granularity)
	t.bucketsMutex.RLock()
	defer t.bucketsMutex.RUnlock()
	return t.isEmptyLocked(now)
}

// isEmptyLocked expects `now` to be truncated and at least Read Lock held.
func (t *RequestsBuckets) isEmptyLocked(now time.Time) bool {
	return now.Sub(t.lastWrite) > t.window
}

func roundToNDigits(n int, f float64) float64 {
	p := math.Pow10(n)
	return math.Round(f*p) / p
}

// WindowAverage returns the average bucket value over the window.
//
// If the first write was less than the window length ago, an average is
// returned over the partial window. For example, if firstWrite was 6 seconds
// ago, the average will be over these 6 seconds worth of buckets, even if the
// window is 60s. If a window passes with no data being received, the first
// write time is reset so this behaviour takes effect again.
//
// Similarly, if we have not received recent data, the average is based on a
// partial window. For example, if the window is 60 seconds but we last
// received data 10 seconds ago, the window average will be the average over
// the first 50 seconds.
//
// In other cases, for example if there are gaps in the data shorter than the
// window length, the missing data is assumed to be 0 and the average is over
// the whole window length inclusive of the missing data.
func (t *RequestsBuckets) WindowAverage(now time.Time) float64 {
	now = now.Truncate(t.granularity)
	t.bucketsMutex.RLock()
	defer t.bucketsMutex.RUnlock()
	switch d := now.Sub(t.lastWrite); {
	case d <= 0:
		// If LastWrite equal or greater than Now
		// return the current WindowTotal, divided by the
		// number of valid buckets
		numB := math.Min(
			float64(t.lastWrite.Sub(t.firstWrite)/t.granularity)+1, // +1 since the times are inclusive.
			float64(len(t.buckets)))
		return roundToNDigits(precision, float64(t.windowTotal)/numB)
	case d < t.window:
		// If we haven't received metrics for some time, which is less than
		// the window -- remove the outdated items and divide by the number
		// of valid buckets
		stIdx := t.timeToIndex(t.lastWrite)
		eIdx := t.timeToIndex(now)
		ret := t.windowTotal
		for i := stIdx + 1; i <= eIdx; i++ {
			ret -= t.buckets[i%len(t.buckets)]
		}
		numB := math.Min(
			float64(t.lastWrite.Sub(t.firstWrite)/t.granularity)+1, // +1 since the times are inclusive.
			float64(len(t.buckets)))
		return roundToNDigits(precision, float64(ret)/numB)
	default: // Nothing for more than a window time, just 0.
		return 0.
	}
}

// timeToIndex converts time to an integer that can be used for modulo
// operations to find the index in the bucket list.
// bucketMutex needs to be held.
func (t *RequestsBuckets) timeToIndex(tm time.Time) int {
	// I don't think this run in 2038 :-)
	// NB: we need to divide by granularity, since it's a compressing mapping
	// to buckets.
	return int(tm.Unix()) / int(t.granularity.Seconds())
}

// Record adds a value with an associated time to the correct bucket.
// If this record would introduce a gap in the data, any intervening times
// between the last write and this one will be recorded as zero. If an entire
// window length has expired without data, the firstWrite time is reset,
// meaning the WindowAverage will be of a partial window until enough data is
// received to fill it again.
func (t *RequestsBuckets) Record(now time.Time, value int) {
	bucketTime := now.Truncate(t.granularity)

	t.bucketsMutex.Lock()
	defer t.bucketsMutex.Unlock()

	writeIdx := t.timeToIndex(now)

	if t.lastWrite != bucketTime {
		if bucketTime.Add(t.window).After(t.lastWrite) {
			// If it is the first write or it happened before the first write which we
			// have in record, update the firstWrite.
			if t.firstWrite.IsZero() || t.firstWrite.After(bucketTime) {
				t.firstWrite = bucketTime
			}

			if bucketTime.After(t.lastWrite) {
				if bucketTime.Sub(t.lastWrite) >= t.window {
					// This means we had no writes for the duration of `window`. So reset the firstWrite time.
					t.firstWrite = bucketTime
					// Reset all the buckets.
					for i := range t.buckets {
						t.buckets[i] = 0
					}
					t.windowTotal = 0
				} else {
					// In theory we might lose buckets between stats gathering.
					// Thus we need to clean not only the current index, but also
					// all the ones from the last write. This is slower than the loop above
					// due to possible wrap-around, so they are not merged together.
					for i := t.timeToIndex(t.lastWrite) + 1; i <= writeIdx; i++ {
						idx := i % len(t.buckets)
						t.windowTotal -= t.buckets[idx]
						t.buckets[idx] = 0
					}
				}
				// Update the last write time.
				t.lastWrite = bucketTime
			}
			// The else case is t.lastWrite - t.window < bucketTime < t.lastWrite, we can simply add
			// the value to the bucket.
		} else {
			// Ignore this value because it happened a window size ago.
			return
		}
	}
	t.buckets[writeIdx%len(t.buckets)] += value
	t.windowTotal += value
}
