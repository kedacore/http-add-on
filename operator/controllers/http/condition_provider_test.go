package http

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func newTestHTTPSO() *httpv1alpha1.HTTPScaledObject {
	return &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Status:     httpv1alpha1.HTTPScaledObjectStatus{Conditions: []httpv1alpha1.HTTPScaledObjectCondition{}},
	}
}

func TestAddOrUpdateCondition(t *testing.T) {
	tests := map[string]struct {
		initial     []httpv1alpha1.HTTPScaledObjectCondition
		updates     []httpv1alpha1.HTTPScaledObjectCondition
		wantStatus  metav1.ConditionStatus
		wantReason  httpv1alpha1.HTTPScaledObjectConditionReason
		wantMessage string
	}{
		"add new condition": {
			initial: nil,
			updates: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionUnknown, Reason: httpv1alpha1.PendingCreation},
			},
			wantStatus: metav1.ConditionUnknown,
			wantReason: httpv1alpha1.PendingCreation,
		},
		"update existing condition replaces all fields": {
			initial: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionUnknown, Reason: httpv1alpha1.PendingCreation, Message: "Initial"},
			},
			updates: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionTrue, Reason: httpv1alpha1.AppScaledObjectCreated, Message: "Updated"},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  httpv1alpha1.AppScaledObjectCreated,
			wantMessage: "Updated",
		},
		"multiple updates keep only last no duplicates": {
			initial: nil,
			updates: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionUnknown, Reason: httpv1alpha1.PendingCreation},
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionTrue, Reason: httpv1alpha1.HTTPScaledObjectIsReady},
			},
			wantStatus: metav1.ConditionTrue,
			wantReason: httpv1alpha1.HTTPScaledObjectIsReady,
		},
		"cleans up existing duplicates": {
			initial: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionUnknown, Reason: httpv1alpha1.PendingCreation},
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionTrue, Reason: httpv1alpha1.AppScaledObjectCreated},
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionTrue, Reason: httpv1alpha1.HTTPScaledObjectIsReady},
			},
			updates: []httpv1alpha1.HTTPScaledObjectCondition{
				{Type: httpv1alpha1.Ready, Status: metav1.ConditionTrue, Reason: httpv1alpha1.HTTPScaledObjectIsReady, Message: "Clean"},
			},
			wantStatus:  metav1.ConditionTrue,
			wantReason:  httpv1alpha1.HTTPScaledObjectIsReady,
			wantMessage: "Clean",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			httpso := newTestHTTPSO()
			httpso.Status.Conditions = tt.initial

			for _, update := range tt.updates {
				httpso = AddOrUpdateCondition(httpso, update)
			}

			if got := len(httpso.Status.Conditions); got != 1 {
				t.Fatalf("len(conditions) = %d, want 1", got)
			}

			cond := httpso.Status.Conditions[0]
			if cond.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s", cond.Status, tt.wantStatus)
			}
			if cond.Reason != tt.wantReason {
				t.Errorf("reason = %s, want %s", cond.Reason, tt.wantReason)
			}
			if tt.wantMessage != "" && cond.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", cond.Message, tt.wantMessage)
			}
		})
	}
}
