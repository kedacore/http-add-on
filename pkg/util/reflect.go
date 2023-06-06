package util

import (
	"reflect"
)

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}

	switch v := reflect.ValueOf(i); v.Kind() {
	case
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.UnsafePointer:
		return v.IsNil()
	}

	return false
}
