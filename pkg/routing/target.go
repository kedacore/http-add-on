package routing

import (
	"errors"
	"fmt"
	"net/url"
)

var ErrTargetNotFound = errors.New("Target not found")

type Target struct {
	Service    string `json:"service"`
	Port       int    `json:"port"`
	Deployment string `json:"deployment"`
}

func NewTarget(svc string, port int, depl string) Target {
	return Target{
		Service:    svc,
		Port:       port,
		Deployment: depl,
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
