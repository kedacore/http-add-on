package routing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTableJSONRoundTrip(t *testing.T) {
	const host = "testhost"
	r := require.New(t)
	tbl := NewTable()
	tgt := Target{
		Service:    "testsvc",
		Port:       8082,
		Deployment: "testdepl",
	}
	tbl.AddTarget(host, tgt)

	b, err := json.Marshal(&tbl)
	r.NoError(err)

	returnTbl := NewTable()
	r.NoError(json.Unmarshal(b, returnTbl))
	retTarget, err := returnTbl.Lookup(host)
	r.NoError(err)
	r.Equal(tgt.Service, retTarget.Service)
	r.Equal(tgt.Port, retTarget.Port)
	r.Equal(tgt.Deployment, retTarget.Deployment)
}

func TestTableRemove(t *testing.T) {
	const host = "testrm"
	r := require.New(t)
	tgt := Target{
		Service:    "testrm",
		Port:       8084,
		Deployment: "testrmdepl",
	}

	tbl := NewTable()

	// add the target to the table and ensure that you can look it up
	tbl.AddTarget(host, tgt)
	retTgt, err := tbl.Lookup(host)
	r.Equal(tgt, retTgt)
	r.NoError(err)

	// remove the target and ensure that you can't look it up
	r.NoError(tbl.RemoveTarget(host))
	retTgt, err = tbl.Lookup(host)
	r.Equal(Target{}, retTgt)
	r.Equal(ErrTargetNotFound, err)
}
