package test

import (
	"context"
	"net/http"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

type Table struct {
	Memory map[string]*httpv1beta1.InterceptorRoute
}

func NewTable() *Table {
	return &Table{
		Memory: make(map[string]*httpv1beta1.InterceptorRoute),
	}
}

var _ routing.Table = (*Table)(nil)

func (t Table) Signal() {
}

func (t Table) Start(_ context.Context) error {
	return nil
}

func (t Table) Route(req *http.Request) *httpv1beta1.InterceptorRoute {
	return t.Memory[req.Host]
}

func (t Table) HasSynced() bool {
	return true
}

var _ util.HealthChecker = (*Table)(nil)

func (t Table) HealthCheck(_ context.Context) error {
	return nil
}
