package routing

import (
	"fmt"
	"net/url"
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix/v2"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

type TableMemory interface {
	Remember(*httpv1alpha1.HTTPScaledObject) TableMemory
	Forget(*httpv1alpha1.HTTPScaledObject) TableMemory
	Recall(*httpv1alpha1.HTTPScaledObject) *httpv1alpha1.HTTPScaledObject
	Route(url *url.URL) *httpv1alpha1.HTTPScaledObject
}

type tableMemory struct {
	tree *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
}

func NewTableMemory() TableMemory {
	return tableMemory{
		tree: iradix.New[*httpv1alpha1.HTTPScaledObject](),
	}
}

var _ TableMemory = (*tableMemory)(nil)

func (tm tableMemory) Remember(httpso *httpv1alpha1.HTTPScaledObject) TableMemory {
	key := tm.treeKeyForHTTPSO(httpso)
	tree, _, _ := tm.tree.Insert(key, httpso)
	return tableMemory{tree}
}

func (tm tableMemory) Forget(httpso *httpv1alpha1.HTTPScaledObject) TableMemory {
	key := tm.treeKeyForHTTPSO(httpso)
	tree, _, _ := tm.tree.Delete(key)
	return tableMemory{tree}
}

func (tm tableMemory) Recall(newHTTPSO *httpv1alpha1.HTTPScaledObject) *httpv1alpha1.HTTPScaledObject {
	key := tm.treeKeyForHTTPSO(newHTTPSO)
	curHTTPSO, _ := tm.tree.Get(key)
	return curHTTPSO
}

func (tm tableMemory) Route(url *url.URL) *httpv1alpha1.HTTPScaledObject {
	key := tm.treeKeyForURL(url)
	_, curHTTPSO, _ := tm.tree.Root().LongestPrefix(key)
	return curHTTPSO
}

func (tm tableMemory) treeKeyForURL(url *url.URL) []byte {
	if url == nil {
		return nil
	}
	return tm.treeKey(url.Host, url.Path)
}

func (tm tableMemory) treeKeyForHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) []byte {
	if httpso == nil {
		return nil
	}
	return tm.treeKey(httpso.Spec.Host, "" /* httpso.Spec.Path */)
}

func (tm tableMemory) treeKey(host string, path string) []byte {
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}

	for strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	if path != "" {
		path = "/" + path
	}

	key := fmt.Sprintf("//%s%s", host, path)
	return []byte(key)
}
