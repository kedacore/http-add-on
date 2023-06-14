package routing

import (
	"k8s.io/client-go/tools/cache"
)

type sharedIndexInformer interface {
	cache.SharedIndexInformer
	HasStarted() bool
}
