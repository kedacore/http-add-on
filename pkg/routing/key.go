package routing

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

type Key []byte

func NewKey(host string, path string) Key {
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	path = strings.Trim(path, "/")
	if path != "" {
		path += "/"
	}

	key := fmt.Sprintf("//%s/%s", host, path)
	return []byte(key)
}

func NewKeyFromURL(url *url.URL) Key {
	if url == nil {
		return nil
	}

	return NewKey(url.Host, url.Path)
}

func NewKeyFromRequest(req *http.Request) Key {
	if req == nil {
		return nil
	}

	reqURL := req.URL
	if reqURL == nil {
		return nil
	}

	keyURL := *reqURL
	if reqHost := req.Host; reqHost != "" {
		keyURL.Host = reqHost
	}

	return NewKeyFromURL(&keyURL)
}

var _ fmt.Stringer = (*Key)(nil)

func (k Key) String() string {
	return string(k)
}

type Keys []Key

func NewKeysFromHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) Keys {
	if httpso == nil {
		return nil
	}
	spec := httpso.Spec

	// TODO(pedrotorres): delete this when we support multiple hosts
	return []Key{
		// TODO(pedrotorres): delete this when we support path prefix
		NewKey(spec.Host, ""),
		// TODO(pedrotorres): uncomment this when we support path prefix
		// NewKey(spec.Host, spec.PathPrefix),
	}

	// TODO(pedrotorres): uncomment this when we support multiple hosts
	//
	// size := len(spec.Hosts)
	// keys := make([]Key, size)
	// for i := 0; i < size; i++ {
	// 	host := spec.Hosts[i]
	// 	path := spec.Paths[i]
	// 	keys[i] = NewKey(host, path)
	// }
	//
	// return keys
}
