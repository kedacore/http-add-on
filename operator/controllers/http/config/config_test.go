package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalScalerHostName(t *testing.T) {
	r := require.New(t)
	sc := ExternalScaler{
		ServiceName: "TestExternalScalerHostNameSvc",
		Port:        int32(8098),
	}
	const ns = "testns"
	hst := sc.HostName(ns)
	spl := strings.Split(hst, ".")
	r.Len(spl, 2, "HostName should return a hostname with 2 parts")
	r.Equal(sc.ServiceName, spl[0])
	r.Equal(fmt.Sprintf("%s:%d", ns, sc.Port), spl[1])
}
