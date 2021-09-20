package k8s

// Int32P converts an int32 into an *int32. It's a convenience function
// for various values in Kubernetes API types
func Int32P(i int32) *int32 {
	return &i
}
