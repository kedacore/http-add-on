package http

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMigrateConditions(t *testing.T) {
	now := metav1.NewTime(time.Now())

	tests := map[string]struct {
		input []metav1.Condition
		want  int
	}{
		"nil input": {nil, 0},
		"old format only": {[]metav1.Condition{
			{Type: "Ready", Reason: "Old1"},
			{Type: "Ready", Reason: "Old2"},
		}, 0},
		"new format only": {[]metav1.Condition{
			{Type: "Ready", Reason: "New", LastTransitionTime: now},
		}, 1},
		"mixed formats": {[]metav1.Condition{
			{Type: "Ready", Reason: "Old"},
			{Type: "Ready", Reason: "New", LastTransitionTime: now},
		}, 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := MigrateConditions(tt.input)
			if len(got) != tt.want {
				t.Errorf("got %d conditions, want %d", len(got), tt.want)
			}
		})
	}
}
