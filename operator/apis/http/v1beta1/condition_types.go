package v1beta1

// Condition type for InterceptorRoute.
const (
	// ConditionTypeReady indicates whether the InterceptorRoute is ready.
	ConditionTypeReady = "Ready"
)

// Condition reasons for InterceptorRoute.
const (
	// ConditionReasonReconciled indicates reconciliation completed successfully.
	ConditionReasonReconciled = "Reconciled"

	// ConditionReasonScaledObjectSyncError indicates that the controller failed
	// to propagate scaling metric changes to one or more ScaledObjects.
	ConditionReasonScaledObjectSyncError = "ScaledObjectSyncError"
)
