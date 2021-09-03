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

func TestTableReplace(t *testing.T) {
	r := require.New(t)
	const host1 = "testreplhost1"
	const host2 = "testreplhost2"
	tgt1 := Target{
		Service:    "tgt1",
		Port:       9090,
		Deployment: "depl1",
	}
	tgt2 := Target{
		Service:    "tgt2",
		Port:       9091,
		Deployment: "depl2",
	}
	// create two routing tables, each with different targets
	tbl1 := NewTable()
	tbl1.AddTarget(host1, tgt1)
	tbl2 := NewTable()
	tbl2.AddTarget(host2, tgt2)

	// replace the second table with the first and ensure that the tables
	// are now equal
	tbl2.Replace(tbl1)

	r.Equal(tbl1, tbl2)
}
