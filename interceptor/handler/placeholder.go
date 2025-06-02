package handler

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/routing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const defaultPlaceholderTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Service Starting</title>
    <meta http-equiv="refresh" content="{{.RefreshInterval}}">
    <meta charset="utf-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            margin: 0;
            background: #f5f5f5;
        }
        .container {
            text-align: center;
            padding: 2rem;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            max-width: 400px;
        }
        h1 {
            color: #333;
            margin-bottom: 1rem;
            font-size: 1.5rem;
        }
        .spinner {
            width: 40px;
            height: 40px;
            margin: 1.5rem auto;
            border: 4px solid #f3f3f3;
            border-top: 4px solid #3498db;
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        p {
            color: #666;
            margin: 0.5rem 0;
        }
        .small {
            font-size: 0.875rem;
            color: #999;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{.ServiceName}} is starting up...</h1>
        <div class="spinner"></div>
        <p>Please wait while we prepare your service.</p>
        <p class="small">This page will refresh automatically every {{.RefreshInterval}} seconds.</p>
    </div>
</body>
</html>`

// cacheEntry stores a template along with resource generation info for cache invalidation
type cacheEntry struct {
	template         *template.Template
	hsoGeneration    int64
	configMapVersion string
}

// PlaceholderHandler handles serving placeholder pages during scale-from-zero
type PlaceholderHandler struct {
	k8sClient     kubernetes.Interface
	routingTable  routing.Table
	templateCache map[string]*cacheEntry
	cacheMutex    sync.RWMutex
	defaultTmpl   *template.Template
}

// PlaceholderData contains data for rendering placeholder templates
type PlaceholderData struct {
	ServiceName     string
	Namespace       string
	RefreshInterval int32
	RequestID       string
	Timestamp       string
}

// NewPlaceholderHandler creates a new placeholder handler
func NewPlaceholderHandler(k8sClient kubernetes.Interface, routingTable routing.Table) (*PlaceholderHandler, error) {
	defaultTmpl, err := template.New("default").Parse(defaultPlaceholderTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default template: %w", err)
	}

	return &PlaceholderHandler{
		k8sClient:     k8sClient,
		routingTable:  routingTable,
		templateCache: make(map[string]*cacheEntry),
		defaultTmpl:   defaultTmpl,
	}, nil
}

// ServePlaceholder serves a placeholder page based on the HTTPScaledObject configuration
func (h *PlaceholderHandler) ServePlaceholder(w http.ResponseWriter, r *http.Request, hso *v1alpha1.HTTPScaledObject) error {
	if hso.Spec.PlaceholderConfig == nil || !hso.Spec.PlaceholderConfig.Enabled {
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return nil
	}

	config := hso.Spec.PlaceholderConfig

	statusCode := int(config.StatusCode)
	if statusCode == 0 {
		statusCode = http.StatusServiceUnavailable
	}

	for k, v := range config.Headers {
		w.Header().Set(k, v)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-KEDA-HTTP-Placeholder-Served", "true")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	tmpl, err := h.getTemplate(r.Context(), hso)
	if err != nil {
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "<h1>%s is starting up...</h1><meta http-equiv='refresh' content='%d'>",
			hso.Spec.ScaleTargetRef.Service, config.RefreshInterval)
		return nil
	}

	data := PlaceholderData{
		ServiceName:     hso.Spec.ScaleTargetRef.Service,
		Namespace:       hso.Namespace,
		RefreshInterval: config.RefreshInterval,
		RequestID:       r.Header.Get("X-Request-ID"),
		Timestamp:       time.Now().Format(time.RFC3339),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "<h1>%s is starting up...</h1><meta http-equiv='refresh' content='%d'>",
			hso.Spec.ScaleTargetRef.Service, config.RefreshInterval)
		return nil
	}

	w.WriteHeader(statusCode)
	_, err = w.Write(buf.Bytes())
	return err
}

// getTemplate retrieves the template for the given HTTPScaledObject
func (h *PlaceholderHandler) getTemplate(ctx context.Context, hso *v1alpha1.HTTPScaledObject) (*template.Template, error) {
	config := hso.Spec.PlaceholderConfig

	if config.Content != "" {
		cacheKey := fmt.Sprintf("%s/%s/inline", hso.Namespace, hso.Name)

		h.cacheMutex.RLock()
		entry, ok := h.templateCache[cacheKey]
		if ok && entry.hsoGeneration == hso.Generation {
			h.cacheMutex.RUnlock()
			return entry.template, nil
		}
		h.cacheMutex.RUnlock()

		tmpl, err := template.New("inline").Parse(config.Content)
		if err != nil {
			return nil, err
		}

		h.cacheMutex.Lock()
		h.templateCache[cacheKey] = &cacheEntry{
			template:      tmpl,
			hsoGeneration: hso.Generation,
		}
		h.cacheMutex.Unlock()
		return tmpl, nil
	}

	if config.ContentConfigMap != "" {
		cacheKey := fmt.Sprintf("%s/%s/cm/%s", hso.Namespace, hso.Name, config.ContentConfigMap)

		cm, err := h.k8sClient.CoreV1().ConfigMaps(hso.Namespace).Get(ctx, config.ContentConfigMap, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get ConfigMap %s: %w", config.ContentConfigMap, err)
		}

		h.cacheMutex.RLock()
		entry, ok := h.templateCache[cacheKey]
		if ok && entry.hsoGeneration == hso.Generation && entry.configMapVersion == cm.ResourceVersion {
			h.cacheMutex.RUnlock()
			return entry.template, nil
		}
		h.cacheMutex.RUnlock()

		key := config.ContentConfigMapKey
		if key == "" {
			key = "template.html"
		}

		content, ok := cm.Data[key]
		if !ok {
			return nil, fmt.Errorf("key %s not found in ConfigMap %s", key, config.ContentConfigMap)
		}

		tmpl, err := template.New("configmap").Parse(content)
		if err != nil {
			return nil, err
		}

		h.cacheMutex.Lock()
		h.templateCache[cacheKey] = &cacheEntry{
			template:         tmpl,
			hsoGeneration:    hso.Generation,
			configMapVersion: cm.ResourceVersion,
		}
		h.cacheMutex.Unlock()
		return tmpl, nil
	}

	return h.defaultTmpl, nil
}
