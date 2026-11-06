package k8s

const (
	ScalerAddressKey    = "scalerAddress"
	InterceptorRouteKey = "interceptorRoute"
	// InterceptorTargetPendingRequestsKey marks the ScaledObject that scales the
	// interceptor fleet. Its value is the target number of pending requests per
	// interceptor replica.
	InterceptorTargetPendingRequestsKey = "interceptorTargetPendingRequests"
)
