package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kedacore/http-add-on/pkg/routing"
)

type testCase struct {
	name      string
	table     routing.TableReader
	counts    map[string]int
	retCounts map[string]int
}

func cases(r *require.Assertions) []testCase {
	return []testCase{
		{
			name: "empty queue",
			table: newRoutingTable(r, []hostAndTarget{
				{
					host:   "www.example.com",
					target: routing.Target{},
				},
				{
					host:   "www.example2.com",
					target: routing.Target{},
				},
			}),
			counts: make(map[string]int),
			retCounts: map[string]int{
				"www.example.com":  0,
				"www.example2.com": 0,
			},
		},
		{
			name: "one entry in queue, same entry in routing table",
			table: newRoutingTable(r, []hostAndTarget{
				{
					host:   "example.com",
					target: routing.Target{},
				},
			}),
			counts: map[string]int{
				"example.com": 1,
			},
			retCounts: map[string]int{
				"example.com": 1,
			},
		},
		{
			name: "one entry in queue, two in routing table",
			table: newRoutingTable(r, []hostAndTarget{
				{
					host:   "example.com",
					target: routing.Target{},
				},
				{
					host:   "example2.com",
					target: routing.Target{},
				},
			}),
			counts: map[string]int{
				"example.com": 1,
			},
			retCounts: map[string]int{
				"example.com":  1,
				"example2.com": 0,
			},
		},
	}
}
func TestGetHostCount(t *testing.T) {
	r := require.New(t)
	for _, tc := range cases(r) {
		for host, retCount := range tc.retCounts {
			t.Run(tc.name, func(t *testing.T) {
				r := require.New(t)
				ret, exists := getHostCount(
					host,
					tc.counts,
					tc.table,
				)
				r.True(exists)
				r.Equal(retCount, ret)
			})
		}
	}
}

type hostAndTarget struct {
	host   string
	target routing.Target
}

func newRoutingTable(r *require.Assertions, entries []hostAndTarget) *routing.Table {
	ret := routing.NewTable()
	for _, entry := range entries {
		r.NoError(ret.AddTarget(entry.host, entry.target))
	}
	return ret
}
