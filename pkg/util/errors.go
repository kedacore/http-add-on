package util

import (
	"context"
	"errors"
	"net/http"
)

var ignoredErrs = []error{
	nil,
	context.Canceled,
	http.ErrServerClosed,
}

func IsIgnoredErr(err error) bool {
	for _, ignoredErr := range ignoredErrs {
		if errors.Is(err, ignoredErr) {
			return true
		}
	}

	return false
}
