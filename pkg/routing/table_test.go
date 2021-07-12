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
