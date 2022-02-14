package main

import "github.com/kedacore/http-add-on/pkg/routing"

// mergeCountsWithRoutingTable ensures that all hosts in
// table are present in counts, and their count is set to 0
// if they weren't already in counts
func mergeCountsWithRoutingTable(
	counts *SafeCount,
	table routing.TableReader,
) map[string]int {
	// ensure that every host is in the queue, even if it has
	// zero pending requests. This is important so that the
	// scaler can report on all applications.

	counts.modifyMut.Lock()
	defer counts.modifyMut.Unlock()

	for _, host := range table.Hosts() {
		_, exists := counts.counts[host]
		if !exists {
			counts.counts[host] = 0
		}
	}
	return counts.counts
}
