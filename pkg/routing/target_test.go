package routing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetServiceURL(t *testing.T) {
	r := require.New(t)

	target := Target{
		Service:    "testsvc",
		Port:       8081,
		Deployment: "testdeploy",
	}
	svcURL, err := target.ServiceURL()
	r.NoError(err)
	r.Equal(
		fmt.Sprintf("%s:%d", target.Service, target.Port),
		svcURL.Host,
	)
}
