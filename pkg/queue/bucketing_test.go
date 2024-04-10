package queue

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const granularity = time.Second

func TestRequestsBucketsSimple(t *testing.T) {
	trunc1 := time.Now().Truncate(1 * time.Second)
	trunc5 := time.Now().Truncate(5 * time.Second)

	type args struct {
		time  time.Time
		value int
	}
	tests := []struct {
		name        string
		granularity time.Duration
		stats       []args
		want        map[time.Time]int
	}{{
		name:        "granularity = 1s",
		granularity: time.Second,
		stats: []args{
			{trunc1, 1.0}, // activator scale from 0.
			{trunc1.Add(100 * time.Millisecond), 10.0}, // from scraping pod/sent by activator.
			{trunc1.Add(1 * time.Second), 1.0},         // next bucket
			{trunc1.Add(3 * time.Second), 1.0},         // nextnextnext bucket
		},
		want: map[time.Time]int{
			trunc1:                      11.0,
			trunc1.Add(1 * time.Second): 1.0,
			trunc1.Add(3 * time.Second): 1.0,
		},
	}, {
		name:        "granularity = 5s",
		granularity: 5 * time.Second,
		stats: []args{
			{trunc5, 1.0},
			{trunc5.Add(3 * time.Second), 11.0}, // same bucket
			{trunc5.Add(6 * time.Second), 1.0},  // next bucket
		},
		want: map[time.Time]int{
			trunc5:                      12.0,
			trunc5.Add(5 * time.Second): 1.0,
		},
	}, {
		name:        "empty",
		granularity: time.Second,
		stats:       []args{},
		want:        map[time.Time]int{},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// New implementation test.
			buckets := NewRequestsBuckets(2*time.Minute, tt.granularity)
			if !buckets.IsEmpty(trunc1) {
				t.Error("Unexpected non empty result")
			}
			for _, stat := range tt.stats {
				buckets.Record(stat.time, stat.value)
			}

			got := make(map[time.Time]int)
			// Less time in future than our window is (2mins above), but more than any of the tests report.
			buckets.forEachBucket(trunc1.Add(time.Minute), func(t time.Time, b int) {
				// Since we're storing 0s when there's no data, we need to exclude those
				// for this test.
				if b > 0 {
					got[t] = b
				}
			})

			if !cmp.Equal(tt.want, got) {
				t.Error("Unexpected values (-want +got):", cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestRequestsBucketsManyReps(t *testing.T) {
	trunc1 := time.Now().Truncate(granularity)
	buckets := NewRequestsBuckets(time.Minute*5, granularity)
	for p := 0; p < 5; p++ {
		trunc1 = trunc1.Add(granularity)
		for t := 0; t < 5; t++ {
			buckets.Record(trunc1, t+p)
		}
	}
	// So the buckets are:
	// t0: [0, 1, 2, 3, 4] = 10
	// t1: [1, 2, 3, 4, 5] = 15
	// t2: [2, 3, 4, 5, 6] = 20
	// t3: [3, 4, 5, 6, 7] = 25
	// t4: [4, 5, 6, 7, 8] = 30
	//                     = 100
	const want = 100
	sum1, sum2 := 0, 0
	buckets.forEachBucket(trunc1, func(_ time.Time, b int) {
		sum1 += b
	})
	buckets.forEachBucket(trunc1, func(_ time.Time, b int) {
		sum2 += b
	})
	if got, want := sum1, want; got != want {
		t.Errorf("Sum1 = %d, want: %d", got, want)
	}

	if got, want := sum2, want; got != want {
		t.Errorf("Sum2 = %d, want: %d", got, want)
	}
}

func TestRequestsBucketsManyRepsWithNonMonotonicalOrder(t *testing.T) {
	start := time.Now().Truncate(granularity)
	end := start
	buckets := NewRequestsBuckets(time.Minute, granularity)

	d := []int{0, 3, 2, 1, 4}
	for p := 0; p < 5; p++ {
		end = start.Add(time.Duration(d[p]) * granularity)
		for t := 0; t < 5; t++ {
			buckets.Record(end, p+t)
		}
	}

	// So the buckets are:
	// t0: [0, 1, 2, 3, 4] = 10
	// t1: [3, 4, 5, 6, 7] = 25
	// t2: [2, 3, 4, 5, 6] = 20
	// t3: [1, 2, 3, 4, 5] = 15
	// t4: [4, 5, 6, 7, 8] = 30
	//                     = 100
	const want = 100
	sum1, sum2 := 0, 0
	buckets.forEachBucket(end, func(_ time.Time, b int) {
		sum1 += b
	})
	buckets.forEachBucket(end, func(_ time.Time, b int) {
		sum2 += b
	})
	if got, want := sum1, want; got != want {
		t.Errorf("Sum1 = %d, want: %d", got, want)
	}

	if got, want := sum2, want; got != want {
		t.Errorf("Sum2 = %d, want: %d", got, want)
	}
}

func TestRequestsBucketsWindowAverage(t *testing.T) {
	now := time.Now()
	buckets := NewRequestsBuckets(5*time.Second, granularity)

	// This verifies that we properly use firstWrite. Without that we'd get 0.2.
	buckets.Record(now, 1)
	if got, want := buckets.WindowAverage(now), 1.; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}
	for i := 1; i < 5; i++ {
		buckets.Record(now.Add(time.Duration(i)*time.Second), i+1)
	}

	if got, want := buckets.WindowAverage(now.Add(4*time.Second)), 15./5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}
	// Check when `now` lags behind.
	if got, want := buckets.WindowAverage(now.Add(3600*time.Millisecond)), 15./5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Check with short hole.
	if got, want := buckets.WindowAverage(now.Add(6*time.Second)), (15.-1-2)/(5-2); got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Check with a long hole.
	if got, want := buckets.WindowAverage(now.Add(10*time.Second)), 0.; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Check write with holes.
	buckets.Record(now.Add(6*time.Second), 91)
	if got, want := buckets.WindowAverage(now.Add(6*time.Second)), (15.-1-2+91)/5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Advance much farther.
	now = now.Add(time.Minute)
	buckets.Record(now, 1984)
	if got, want := buckets.WindowAverage(now), 1984.; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Check with an earlier time.
	buckets.Record(now.Add(-3*time.Second), 4)
	if got, want := buckets.WindowAverage(now), (4.+1984)/4; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// One more second pass.
	now = now.Add(time.Second)
	buckets.Record(now, 5)
	if got, want := buckets.WindowAverage(now), (4.+1984+5)/5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Insert an earlier time again.
	buckets.Record(now.Add(-3*time.Second), 10)
	if got, want := buckets.WindowAverage(now), (4.+10+1984+5)/5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Verify that we ignore the value which is too early.
	buckets.Record(now.Add(-6*time.Second), 10)
	if got, want := buckets.WindowAverage(now), (4.+10+1984+5)/5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Verify that we ignore the value with bound timestamp.
	buckets.Record(now.Add(-5*time.Second), 10)
	if got, want := buckets.WindowAverage(now), (4.+10+1984+5)/5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}

	// Verify we clear up the data when not receiving data for exact `window` peroid.
	buckets.Record(now.Add(5*time.Second), 10)
	if got, want := buckets.WindowAverage(now.Add(5*time.Second)), 10.; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}
}

func TestDescendingRecord(t *testing.T) {
	now := time.Now()
	buckets := NewRequestsBuckets(5*time.Second, 1*time.Second)

	for i := 8 * time.Second; i >= 0*time.Second; i -= time.Second {
		buckets.Record(now.Add(i), 5)
	}

	if got, want := buckets.WindowAverage(now.Add(5*time.Second)), 5.; got != want {
		// we wrote a 5 every second, and we never wrote in the same second twice,
		// so the average _should_ be 5.
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}
}

func TestRequestsBucketsHoles(t *testing.T) {
	now := time.Now()
	buckets := NewRequestsBuckets(5*time.Second, granularity)

	for i := time.Duration(0); i < 5; i++ {
		buckets.Record(now.Add(i*time.Second), int(i+1))
	}

	sum := 0

	buckets.forEachBucket(now.Add(4*time.Second),
		func(_ time.Time, b int) {
			sum += b
		})

	if got, want := sum, 15; got != want {
		t.Errorf("Sum = %v, want: %v", got, want)
	}
	if got, want := buckets.WindowAverage(now.Add(4*time.Second)), 15./5; got != want {
		t.Errorf("WindowAverage = %v, want: %v", got, want)
	}
	// Now write at 9th second. Which means that seconds
	// 5[0], 6[1], 7[2] become 0.
	buckets.Record(now.Add(8*time.Second), 2.)
	// So now we have [3] = 2, [4] = 5 and sum should be 7.
	sum = 0.

	buckets.forEachBucket(now.Add(8*time.Second),
		func(_ time.Time, b int) {
			sum += b
		})
	if got, want := sum, 7; got != want {
		t.Errorf("Sum = %v, want: %v", got, want)
	}
}

func BenchmarkWindowAverage(b *testing.B) {
	// Window lengths in secs.
	for _, wl := range []int{30, 60, 120, 240, 600} {
		b.Run(fmt.Sprintf("%v-win-len", wl), func(b *testing.B) {
			tn := time.Now().Truncate(time.Second) // To simplify everything.
			buckets := NewRequestsBuckets(time.Duration(wl)*time.Second,
				time.Second /*granularity*/)
			// Populate with some random data.
			for i := 0; i < wl; i++ {
				buckets.Record(tn.Add(time.Duration(i)*time.Second), rand.Int()*100)
			}
			for i := 0; i < b.N; i++ {
				buckets.WindowAverage(tn.Add(time.Duration(wl) * time.Second))
			}
		})
	}
}

func TestRoundToNDigits(t *testing.T) {
	if got, want := roundToNDigits(6, 3.6e-17), 0.; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(3, 0.0004), 0.; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(3, 1.2345), 1.234; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(4, 1.2345), 1.2345; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(6, 12345), 12345.; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
}

func (t *RequestsBuckets) forEachBucket(now time.Time, acc func(time time.Time, bucket int)) {
	now = now.Truncate(t.granularity)
	t.bucketsMutex.RLock()
	defer t.bucketsMutex.RUnlock()

	// So number of buckets we can process is len(buckets)-(now-lastWrite)/granularity.
	// Since empty check above failed, we know this is at least 1 bucket.
	numBuckets := len(t.buckets) - int(now.Sub(t.lastWrite)/t.granularity)
	bucketTime := t.lastWrite // Always aligned with granularity.
	si := t.timeToIndex(bucketTime)
	for i := 0; i < numBuckets; i++ {
		tIdx := si % len(t.buckets)
		acc(bucketTime, t.buckets[tIdx])
		si--
		bucketTime = bucketTime.Add(-t.granularity)
	}
}
