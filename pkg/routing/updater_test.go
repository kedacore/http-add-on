package routing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartUpdateLoop(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	const interval = 10 * time.Millisecond

	tbl := NewTable()
	newTbl := NewTable()
	newTbl.AddTarget("foo", Target{
		Service:    "fnsvc",
		Port:       8086,
		Deployment: "fndpl",
	})
	fn := func(ctx context.Context) (*Table, error) {
		return newTbl, nil
	}

	go StartUpdateLoop(ctx, interval, tbl, fn)
	time.Sleep(interval * 2)
	r.Equal(newTbl, tbl)
}
