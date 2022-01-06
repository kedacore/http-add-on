package routing

import (
	"errors"
	"fmt"
	"net/url"
)

// ErrTargetNotFound is returned when a target is not
// found in the table.
var ErrTargetNotFound = errors.New("Target not found")

// Target is a single target in the routing table.
type Target struct {
	Service               string
	Port                  int
	Deployment            string
	Namespace             string
	TargetPendingRequests int32
}

// NewTarget creates a new Target from the given parameters.
func NewTarget(
	namespace,
	svc string,
	port int,
	depl string,
	targetPendingReqs int32,
) Target {
	return Target{
		Service:               svc,
		Port:                  port,
		Deployment:            depl,
		TargetPendingRequests: targetPendingReqs,
		Namespace:             namespace,
	}
}

// ServiceURLFunc is a function that returns the full in-cluster
// URL for the given Target.
//
// ServiceURL is the production implementation of this function
type ServiceURLFunc func(Target) (*url.URL, error)

// ServiceURL returns the full URL for the Kubernetes service,
// port and namespace of t.
func ServiceURL(t Target) (*url.URL, error) {
	urlStr := fmt.Sprintf(
		"http://%s.%s:%d",
		t.Service,
		t.Namespace,
		t.Port,
	)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil
}
