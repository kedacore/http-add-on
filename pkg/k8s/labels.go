package k8s

import "k8s.io/apimachinery/pkg/labels"

// ResponseBodyLabels is the label set that ConfigMaps must carry to be
// included in the interceptor's informer cache.
var ResponseBodyLabels = labels.Set{"http.keda.sh/response-body": "true"}
