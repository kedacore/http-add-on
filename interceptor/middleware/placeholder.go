package middleware

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	headerContentType   = "Content-Type"
	defaultConfigMapKey = "index.html"
)

// Placeholder short-circuits requests with a static response when the
// backend has no ready endpoints and a placeholder response is configured.
// It sits before the EndpointResolver so the caller gets an immediate
// reply instead of blocking until the backend scales up.
type Placeholder struct {
	next       http.Handler
	readyCache *k8s.ReadyEndpointsCache
	reader     client.Reader
}

// NewPlaceholder returns a middleware that serves a static placeholder
// response when the target has no ready endpoints. The reader is used
// to resolve response bodies stored in ConfigMaps.
func NewPlaceholder(next http.Handler, readyCache *k8s.ReadyEndpointsCache, reader client.Reader) *Placeholder {
	return &Placeholder{
		next:       next,
		readyCache: readyCache,
		reader:     reader,
	}
}

var _ http.Handler = (*Placeholder)(nil)

func (p *Placeholder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ir := util.InterceptorRouteFromContext(r.Context())

	if ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Placeholder != nil && ir.Spec.ColdStart.Placeholder.Response != nil {
		serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
		if !p.readyCache.HasReadyEndpoints(serviceKey) {
			p.serveStaticResponse(w, r, ir, ir.Spec.ColdStart.Placeholder.Response)
			return
		}
	}

	p.next.ServeHTTP(w, r)
}

// serveStaticResponse writes an HTTP response from the StaticResponse spec.
func (p *Placeholder) serveStaticResponse(w http.ResponseWriter, r *http.Request, ir *httpv1beta1.InterceptorRoute, resp *httpv1beta1.StaticResponse) {
	body, contentType, err := p.resolveBody(r, ir, resp)
	if err != nil {
		logger := util.LoggerFromContext(r.Context())
		logger.Error(err, "failed to resolve placeholder body",
			"interceptorRoute", k8s.NamespacedNameFromObject(ir),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	if contentType != "" && w.Header().Get(headerContentType) == "" {
		w.Header().Set(headerContentType, contentType)
	}

	statusCode := int(resp.StatusCode)
	if statusCode == 0 {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)

	if body != "" {
		if _, err := io.WriteString(w, body); err != nil {
			logger := util.LoggerFromContext(r.Context())
			logger.Error(err, "failed to write placeholder response body",
				"namespacedName", k8s.NamespacedNameFromObject(ir),
			)
		}
	}
}

// resolveBody returns the response body and an optional auto-detected mime type.
func (p *Placeholder) resolveBody(r *http.Request, ir *httpv1beta1.InterceptorRoute, resp *httpv1beta1.StaticResponse) (string, string, error) {
	if resp.Body != nil {
		return *resp.Body, "", nil
	}
	if resp.BodyFromConfigMap == nil {
		return "", "", nil
	}

	ref := resp.BodyFromConfigMap
	var cm corev1.ConfigMap
	err := p.reader.Get(r.Context(), types.NamespacedName{
		Namespace: ir.Namespace,
		Name:      ref.Name,
	}, &cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", "", fmt.Errorf(
				"ConfigMap %s/%s not found: ensure it exists and has the label %s",
				ir.Namespace, ref.Name, k8s.ResponseBodyLabels,
			)
		}
		return "", "", fmt.Errorf("failed to get ConfigMap %s/%s: %w", ir.Namespace, ref.Name, err)
	}

	key := ref.Key
	if key == "" {
		key = configMapKeyFromPath(r.URL.Path)
	}

	val, ok := cm.Data[key]
	if !ok {
		// Explicit key miss is an error; path-derived key miss returns an empty body
		// so that missing sub-resources (e.g. /favicon.ico) degrade gracefully.
		if ref.Key != "" {
			return "", "", fmt.Errorf("key %q not found in ConfigMap %s/%s", key, ir.Namespace, ref.Name)
		}
		return "", "", nil
	}

	ct := mime.TypeByExtension(path.Ext(key))
	return val, ct, nil
}

// configMapKeyFromPath derives a ConfigMap key from the request path.
// ConfigMap keys cannot contain "/", so nested paths won't match.
func configMapKeyFromPath(urlPath string) string {
	key := strings.TrimPrefix(urlPath, "/")
	if key == "" {
		return defaultConfigMapKey
	}
	return key
}
