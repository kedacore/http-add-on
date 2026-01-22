package v1alpha1

// Condition type for HTTPScaledObject.
const (
	// ConditionTypeReady indicates whether the HTTPScaledObject is ready.
	ConditionTypeReady = "Ready"
)

// Condition reasons for HTTPScaledObject.
const (
	// ConditionReasonReconciled indicates reconciliation completed successfully.
	ConditionReasonReconciled = "Reconciled"
	// ConditionReasonCreateError indicates an error occurred during creation.
	ConditionReasonCreateError = "CreateError"
)
