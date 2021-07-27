package routing

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/require"
)

func TestRPCIntegration(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	table := NewTable()
	targets := []Target{
		{
			Service:    "svc1",
			Port:       1234,
			Deployment: "depl1",
		},
		{
			Service:    "svc2",
			Port:       2345,
			Deployment: "depl2",
		},
	}

	hdl := kedanet.NewTestHTTPHandlerWrapper(newTableHandler(
		logr.Discard(),
		table,
	))
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	// iterate the targets and successively add each target each iteration.
	// before we start iterating, there should be no targets
	err = GetTable(
		ctx,
		srv.Client(),
		logr.Discard(),
		*url,
		table,
		queue.NewFakeCounter(),
	)
	r.NoError(err)
	r.Equal(0, len(table.m))
	for i, target := range targets {
		host := fmt.Sprintf("host%d", i)
		table.AddTarget(host, target)

		// for this iteration, we should have i+1 table entries
		err := GetTable(
			ctx,
			srv.Client(),
			logr.Discard(),
			*url,
			table,
			queue.NewFakeCounter(),
		)
		r.NoError(err)
		r.Equal(i+1, len(table.m))

		// iterate from [0, i] (i.e. from 0 up to i, inclusive on each side of
		// the range) and for each index, reconstruct the hostname and
		// look it up in the response
		for hostIdx := 0; hostIdx <= i; hostIdx++ {
			lookupHost := fmt.Sprintf("host%d", hostIdx)
			retVal, ok := table.m[lookupHost]
			r.True(ok, "host %s not found in returned table", lookupHost)
			r.Equal(table.m[lookupHost], retVal)
		}
	}
}
