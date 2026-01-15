package net

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestMinTotalBackoffDuration(t *testing.T) {
	tests := map[string]struct {
		backoff wait.Backoff
		want    time.Duration
	}{
		"without cap": {
			backoff: wait.Backoff{
				Duration: 100 * time.Millisecond,
				Factor:   2,
				Steps:    3,
			},
			want: 700 * time.Millisecond, // 100 + 200 + 400
		},
		"with cap limiting later steps": {
			backoff: wait.Backoff{
				Duration: 100 * time.Millisecond,
				Factor:   2,
				Steps:    4,
				Cap:      250 * time.Millisecond,
			},
			want: 800 * time.Millisecond, // 100 + 200 + 250 + 250
		},
		"with cap never reached": {
			backoff: wait.Backoff{
				Duration: 100 * time.Millisecond,
				Factor:   2,
				Steps:    3,
				Cap:      1 * time.Second,
			},
			want: 700 * time.Millisecond, // 100 + 200 + 400
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := MinTotalBackoffDuration(tt.backoff)
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}
