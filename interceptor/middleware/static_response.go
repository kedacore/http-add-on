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

// serveStaticResponse writes an HTTP response from the StaticResponse spec.
func serveStaticResponse(w http.ResponseWriter, r *http.Request, reader client.Reader, ir *httpv1beta1.InterceptorRoute, resp *httpv1beta1.StaticResponse, defaultStatusCode int) {
	body, contentType, err := resolveBody(r, reader, ir, resp)
	if err != nil {
		logger := util.LoggerFromContext(r.Context())
		logger.Error(err, "failed to resolve static response body",
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
		statusCode = defaultStatusCode
	}
	w.WriteHeader(statusCode)

	if body != "" {
		if _, err := io.WriteString(w, body); err != nil {
			logger := util.LoggerFromContext(r.Context())
			logger.Error(err, "failed to write static response body",
				"namespacedName", k8s.NamespacedNameFromObject(ir),
			)
		}
	}
}

// resolveBody returns the response body and an optional auto-detected mime type.
func resolveBody(r *http.Request, reader client.Reader, ir *httpv1beta1.InterceptorRoute, resp *httpv1beta1.StaticResponse) (string, string, error) {
	if resp.Body != nil {
		return *resp.Body, "", nil
	}
	if resp.BodyFromConfigMap == nil {
		return "", "", nil
	}

	ref := resp.BodyFromConfigMap
	var cm corev1.ConfigMap
	err := reader.Get(r.Context(), types.NamespacedName{
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
