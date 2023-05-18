package main

import (
	"fmt"
	"regexp"

	"github.com/kedacore/http-add-on/pkg/routing"
)

var (
	unsafeChars = regexp.MustCompile(`[^-.0-9A-Za-z]`)
)

func escapeRune(r string) string {
	return fmt.Sprintf("_%04X", r)
}

func escapeString(s string) string {
	return unsafeChars.ReplaceAllStringFunc(s, escapeRune)
}

func MetricName(host string, pathPrefix string) string {
	rk := routing.NewKey(host, pathPrefix)
	mn := fmt.Sprintf("http-%v", rk)
	return escapeString(mn)
}
