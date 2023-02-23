package k8s

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
)

func ObjectKind(obj runtime.Object) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
