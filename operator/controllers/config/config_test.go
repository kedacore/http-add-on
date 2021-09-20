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
	r.Equal(5, len(spl), "HostName should return a hostname with 5 parts")
	r.Equal(sc.ServiceName, spl[0])
	r.Equal(ns, spl[1])
	r.Equal("svc", spl[2])
	r.Equal("cluster", spl[3])
	r.Equal(fmt.Sprintf("local:%d", sc.Port), spl[4])

}
