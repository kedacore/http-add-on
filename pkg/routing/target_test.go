package routing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetServiceURL(t *testing.T) {
	r := require.New(t)

	target := NewTarget(
		"testns",
		"testsvc",
		8081,
		"testdeploy",
		1234,
	)
	svcURL, err := ServiceURL(target)
	r.NoError(err)
	r.Equal(
		fmt.Sprintf("%s.%s:%d", target.Service, target.Namespace, target.Port),
		svcURL.Host,
	)
}
