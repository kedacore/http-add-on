package queue

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
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
	for p := range 5 {
		trunc1 = trunc1.Add(granularity)
		for t := range 5 {
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
	for p := range 5 {
		end = start.Add(time.Duration(d[p]) * granularity)
		for t := range 5 {
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
	if got, want := buckets.WindowAverage(now.Add(6*time.Second)), (15.-1-2)/(5); got != want {
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

func TestRequestsSuddenStop(t *testing.T) {
	RegisterTestingT(t)
	now := time.Date(2024, 6, 26, 12, 0, 0, 0, time.UTC)
	buckets := NewRequestsBuckets(5*time.Second, granularity)

	// empty window
	Expect(buckets.buckets).To(Equal([]int{0, 0, 0, 0, 0}))
	Expect(buckets.WindowAverage(now)).To(Equal(0.0))

	// first bucket with 1 request
	buckets.Record(now, 1)
	Expect(buckets.WindowAverage(now)).To(Equal(1.0))
	Expect(buckets.buckets).To(Equal([]int{1, 0, 0, 0, 0}))

	// second bucket with 2 requests
	buckets.Record(now.Add(1*time.Second), 2)
	Expect(buckets.WindowAverage(now.Add(1 * time.Second))).To(Equal(1.5))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 0, 0, 0}))

	// third bucket with 3 requests
	buckets.Record(now.Add(2*time.Second), 3)
	Expect(buckets.WindowAverage(now.Add(2 * time.Second))).To(Equal(2.0))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 3, 0, 0}))

	// fourth bucket with 4 requests
	buckets.Record(now.Add(3*time.Second), 4)
	Expect(buckets.WindowAverage(now.Add(3 * time.Second))).To(Equal(2.5))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 3, 4, 0}))

	// fifth bucket with 5 requests
	buckets.Record(now.Add(4*time.Second), 5)
	Expect(buckets.WindowAverage(now.Add(4 * time.Second))).To(Equal(3.0))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 3, 4, 5}))

	// first bucket (sixth time window), we don't have any requests, so the average should be 0+2+3+4+5/5 = 2.8
	// but the buckets don't change until new value is recorded or until the window expires
	Expect(buckets.WindowAverage(now.Add(5 * time.Second))).To(Equal(2.8))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 3, 4, 5}))

	// second bucket, also no requests
	Expect(buckets.WindowAverage(now.Add(6 * time.Second))).To(Equal(2.4))
	Expect(buckets.buckets).To(Equal([]int{1, 2, 3, 4, 5}))

	// third bucket, 8 requests
	buckets.Record(now.Add(7*time.Second), 8)
	Expect(buckets.WindowAverage(now.Add(7 * time.Second))).To(Equal(3.4))
	Expect(buckets.buckets).To(Equal([]int{0, 0, 8, 4, 5}))

	// fourth bucket, 9 requests
	buckets.Record(now.Add(8*time.Second), 9)
	Expect(buckets.WindowAverage(now.Add(8 * time.Second))).To(Equal(4.4))
	Expect(buckets.buckets).To(Equal([]int{0, 0, 8, 9, 5}))

	// fifth bucket, 10 requests
	buckets.Record(now.Add(9*time.Second), 10)
	Expect(buckets.WindowAverage(now.Add(9 * time.Second))).To(Equal(5.4))
	Expect(buckets.buckets).To(Equal([]int{0, 0, 8, 9, 10}))

	// first bucket, 11 requests
	buckets.Record(now.Add(10*time.Second), 11)
	Expect(buckets.WindowAverage(now.Add(10 * time.Second))).To(Equal(7.6))
	Expect(buckets.buckets).To(Equal([]int{11, 0, 8, 9, 10}))

	// second bucket, 12 requests
	buckets.Record(now.Add(11*time.Second), 12)
	Expect(buckets.WindowAverage(now.Add(11 * time.Second))).To(Equal(10.0))
	Expect(buckets.buckets).To(Equal([]int{11, 12, 8, 9, 10}))

	// now requests stop entirely and time window average decreases all the way to 0
	Expect(buckets.WindowAverage(now.Add(12 * time.Second))).To(Equal(8.4))
	Expect(buckets.WindowAverage(now.Add(13 * time.Second))).To(Equal(6.6))
	Expect(buckets.WindowAverage(now.Add(14 * time.Second))).To(Equal(4.6))
	Expect(buckets.WindowAverage(now.Add(15 * time.Second))).To(Equal(2.4))
	Expect(buckets.WindowAverage(now.Add(16 * time.Second))).To(Equal(0.0))

	// and single request is recorded after avg dropped to 0, this should restart the window
	buckets.Record(now.Add(17*time.Second), 1)
	Expect(buckets.WindowAverage(now.Add(17 * time.Second))).To(Equal(1.0))
	Expect(buckets.buckets).To(Equal([]int{0, 0, 1, 0, 0}))
}

func TestRequestsBucketsHoles(t *testing.T) {
	now := time.Now()
	buckets := NewRequestsBuckets(5*time.Second, granularity)

	for i := range time.Duration(5) {
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
			for i := range wl {
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
	if got, want := roundToNDigits(3, -3.6e-17), 0.; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(3, 0.0004), 0.; got != want {
		t.Errorf("Rounding = %v, want: %v", got, want)
	}
	if got, want := roundToNDigits(3, 1.2345), 1.235; got != want {
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
	for range numBuckets {
		tIdx := si % len(t.buckets)
		acc(bucketTime, t.buckets[tIdx])
		si--
		bucketTime = bucketTime.Add(-t.granularity)
	}
}
