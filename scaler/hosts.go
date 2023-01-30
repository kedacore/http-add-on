package main

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"gopkg.in/yaml.v2"
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

// gets hosts from scaledobjectref
func getHostsFromScaledObjectRef(lggr logr.Logger, sor *externalscaler.ScaledObjectRef) ([]string, error) {
	serializedHosts, ok := sor.ScalerMetadata["hosts"]
	if !ok {
		err := fmt.Errorf("'hosts' not found in ScaledObject metadata")
		lggr.Error(err, "no 'hosts' found in ScaledObject metadata")
		return make([]string, 0), err
	}
	var hosts []string
	err := yaml.Unmarshal([]byte(serializedHosts), &hosts)
	if err != nil {
		err := fmt.Errorf("unable to unmarshal 'hosts' from scaledobject config")
		lggr.Error(err, "'hosts' not configured properly in scaledobjectref")
		return make([]string, 0), err
	}
	return hosts, nil
}
