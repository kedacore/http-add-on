package test

import (
	"context"
	"net/http"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/routing"
)

type Table struct {
	Memory map[string]*httpv1alpha1.HTTPScaledObject
}

func NewTable() *Table {
	return &Table{
		Memory: make(map[string]*httpv1alpha1.HTTPScaledObject),
	}
}

var _ routing.Table = (*Table)(nil)

func (t Table) Start(_ context.Context) error {
	return nil
}

func (t Table) Route(req *http.Request) *httpv1alpha1.HTTPScaledObject {
	return t.Memory[req.Host]
}

func (t Table) HasSynced() bool {
	return true
}
