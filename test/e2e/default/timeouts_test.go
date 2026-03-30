//go:build e2e

package default_test

import (
	"testing"
)

func TestTimeouts(t *testing.T) {
	t.Skip("blocked on InterceptorRoute timeout support, see https://github.com/kedacore/http-add-on/issues/1474")
}
