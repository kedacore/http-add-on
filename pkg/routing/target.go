package routing

import (
	"errors"
	"fmt"
	"net/url"
)

var ErrTargetNotFound = errors.New("Target not found")

// Target is the target for a given host. It represents where to forward
// the network request, what deployment fronts the pods that serve that
// network request, and the target number of pending requests to use for
// scaling the deployment
type Target struct {
	Service               string `json:"service"`
	Port                  int    `json:"port"`
	Deployment            string `json:"deployment"`
	TargetPendingRequests int32  `json:"target"`
}

// NewTarget creates a new Target from the given parameters.
func NewTarget(
	svc string,
	port int,
	depl string,
	target int32,
) Target {
	return Target{
		Service:               svc,
		Port:                  port,
		Deployment:            depl,
		TargetPendingRequests: target,
	}
}

func (t *Target) ServiceURL() (*url.URL, error) {
	urlStr := fmt.Sprintf("http://%s:%d", t.Service, t.Port)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil

}
