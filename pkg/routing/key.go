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

func NewKeyFromRequest(req *http.Request, httpRoutingHeaderKey string) Key {
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

	if httpRoutingHeaderKey != "" {
		if httpRoutingHeaderValue := req.Header.Get(httpRoutingHeaderKey); httpRoutingHeaderValue != "" {
			keyURL.Host = httpRoutingHeaderValue
		}
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

	hosts := spec.Hosts
	if hosts == nil {
		hosts = []string{""}
	}
	hostsSize := len(hosts)

	pathPrefixes := spec.PathPrefixes
	if pathPrefixes == nil {
		pathPrefixes = []string{""}
	}
	pathPrefixesSize := len(pathPrefixes)

	keysSize := hostsSize * pathPrefixesSize
	keys := make([]Key, 0, keysSize)
	for _, host := range hosts {
		for _, pathPrefix := range pathPrefixes {
			key := NewKey(host, pathPrefix)
			keys = append(keys, key)
		}
	}

	return keys
}
