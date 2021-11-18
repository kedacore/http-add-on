package main

import (
	"github.com/kedacore/http-add-on/pkg/routing"
)

// mergeCountsWithRoutingTable ensures that all hosts in routing table
// are present in combined counts, if count is not present value is set to 0
func mergeCountsWithRoutingTable(
	counts map[string]int,
	table routing.TableReader,
) map[string]int {
	mergedCounts := make(map[string]int)
	for _, host := range table.Hosts() {
		mergedCounts[host] = 0
	}
	for key, value := range counts {
		mergedCounts[key] = value
	}
	return mergedCounts
}

// getHostCount gets proper count for given host regardless whether
// host is in counts or only in routerTable
func getHostCount(
	host string,
	counts map[string]int,
	table routing.TableReader,
) (int, bool) {
	count, exists := counts[host]
	if exists {
		return count, exists
	}

	exists = table.HasHost(host)
	return 0, exists
}
