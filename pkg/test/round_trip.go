// Package test contains helper and utility functions
// for use primarily in tests.
//
// It is generally not a good idea to use anything in this
// package in production code, unless you're very familiar
// with that code and its performance characteristics.
package test

import "encoding/json"

// JSONRoundTrip round trips src to JSON and back
// out into target
//
// This function is primarily intended for translating
// a map to a configuration struct, and intended for use
// in tests.
func JSONRoundTrip(src interface{}, target interface{}) error {
	srcBytes, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(srcBytes, target)
}
