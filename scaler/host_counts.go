package main

import (
	"github.com/kedacore/http-add-on/pkg/routing"
)

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
