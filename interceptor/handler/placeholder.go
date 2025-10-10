package handler

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const (
	headerPlaceholderServed = "X-KEDA-HTTP-Placeholder-Served"
	headerCacheControl      = "Cache-Control"
	headerContentType       = "Content-Type"
	cacheControlValue       = "no-cache, no-store, must-revalidate"
	fallbackContentType     = "text/plain; charset=utf-8"
	fallbackMessageFormat   = "%s is starting up...\n"
)

type cacheEntry struct {
	template      *template.Template
	hsoGeneration int64
}

type PlaceholderHandler struct {
	templateCache map[string]*cacheEntry
	cacheMutex    sync.RWMutex
	defaultTmpl   *template.Template
	servingCfg    *config.Serving
}

type PlaceholderData struct {
	ServiceName     string
	Namespace       string
	RefreshInterval int32
	RequestID       string
	Timestamp       string
}

func NewPlaceholderHandler(servingCfg *config.Serving) (*PlaceholderHandler, error) {
	var defaultTmpl *template.Template

	if servingCfg.PlaceholderDefaultTemplatePath != "" {
		content, err := os.ReadFile(servingCfg.PlaceholderDefaultTemplatePath)
		if err != nil {
			fmt.Printf("Warning: Could not read placeholder template from %s: %v. No default template will be used.\n",
				servingCfg.PlaceholderDefaultTemplatePath, err)
		} else {
			defaultTmpl, err = template.New("default").Parse(string(content))
			if err != nil {
				fmt.Printf("Warning: Could not parse placeholder template from %s: %v. No default template will be used.\n",
					servingCfg.PlaceholderDefaultTemplatePath, err)
				defaultTmpl = nil
			}
		}
	}

	return &PlaceholderHandler{
		templateCache: make(map[string]*cacheEntry),
		defaultTmpl:   defaultTmpl,
		servingCfg:    servingCfg,
	}, nil
}

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

	tmpl, err := h.resolveTemplate(r.Context(), hso)
	if err != nil {
		return h.serveFallbackPlaceholder(w, hso.Spec.ScaleTargetRef.Service, statusCode)
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
		return h.serveFallbackPlaceholder(w, hso.Spec.ScaleTargetRef.Service, statusCode)
	}

	w.Header().Set(headerPlaceholderServed, "true")
	w.Header().Set(headerCacheControl, cacheControlValue)
	w.WriteHeader(statusCode)
	_, err = w.Write(buf.Bytes())
	return err
}

func (h *PlaceholderHandler) serveFallbackPlaceholder(w http.ResponseWriter, serviceName string, statusCode int) error {
	w.Header().Set(headerContentType, fallbackContentType)
	w.Header().Set(headerPlaceholderServed, "true")
	w.Header().Set(headerCacheControl, cacheControlValue)
	w.WriteHeader(statusCode)
	_, err := fmt.Fprintf(w, fallbackMessageFormat, serviceName)
	return err
}

func (h *PlaceholderHandler) resolveTemplate(_ context.Context, hso *v1alpha1.HTTPScaledObject) (*template.Template, error) {
	config := hso.Spec.PlaceholderConfig

	if config.Content != "" {
		return h.getCachedInlineTemplate(hso, config.Content)
	}

	if h.defaultTmpl != nil {
		return h.defaultTmpl, nil
	}

	return nil, fmt.Errorf("no placeholder template configured")
}

func (h *PlaceholderHandler) getCachedInlineTemplate(hso *v1alpha1.HTTPScaledObject, content string) (*template.Template, error) {
	cacheKey := fmt.Sprintf("%s/%s/inline", hso.Namespace, hso.Name)

	h.cacheMutex.RLock()
	entry, ok := h.templateCache[cacheKey]
	if ok && entry.hsoGeneration == hso.Generation {
		h.cacheMutex.RUnlock()
		return entry.template, nil
	}
	h.cacheMutex.RUnlock()

	h.cacheMutex.Lock()
	defer h.cacheMutex.Unlock()

	tmpl, err := template.New("inline").Parse(content)
	if err != nil {
		return nil, err
	}

	h.templateCache[cacheKey] = &cacheEntry{
		template:      tmpl,
		hsoGeneration: hso.Generation,
	}

	return tmpl, nil
}
