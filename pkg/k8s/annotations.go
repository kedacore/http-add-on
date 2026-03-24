package k8s

const (
	// Annotations used to carry HTTPSO per-object timeout overrides through
	// the InterceptorRoute without exposing them in its public spec.
	AnnotationConditionWaitTimeout  = "internal.http.keda.sh/condition-wait-timeout"
	AnnotationResponseHeaderTimeout = "internal.http.keda.sh/response-header-timeout"
)
