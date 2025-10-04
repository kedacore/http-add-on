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


// cacheEntry stores a template along with resource generation info for cache invalidation
type cacheEntry struct {
	template         *template.Template
	hsoGeneration    int64
	configMapVersion string
}

// PlaceholderHandler handles serving placeholder pages during scale-from-zero
type PlaceholderHandler struct {
	templateCache map[string]*cacheEntry
	cacheMutex    sync.RWMutex
	defaultTmpl   *template.Template
	servingCfg    *config.Serving
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
func NewPlaceholderHandler(servingCfg *config.Serving) (*PlaceholderHandler, error) {
	var defaultTmpl *template.Template

	// Try to load template from configured path if provided
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

	// Set custom headers first
	for k, v := range config.Headers {
		w.Header().Set(k, v)
	}

	// Get template and render content
	tmpl, err := h.getTemplate(r.Context(), hso)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-KEDA-HTTP-Placeholder-Served", "true")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "%s is starting up...\n", hso.Spec.ScaleTargetRef.Service)
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-KEDA-HTTP-Placeholder-Served", "true")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "%s is starting up...\n", hso.Spec.ScaleTargetRef.Service)
		return nil
	}

	content := buf.String()

	// Set standard headers
	w.Header().Set("X-KEDA-HTTP-Placeholder-Served", "true")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	w.WriteHeader(statusCode)
	_, err = w.Write([]byte(content))
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

		h.cacheMutex.Lock()
		tmpl, err := template.New("inline").Parse(config.Content)
		if err != nil {
			h.cacheMutex.Unlock()
			return nil, err
		}
		h.templateCache[cacheKey] = &cacheEntry{
			template:      tmpl,
			hsoGeneration: hso.Generation,
		}
		h.cacheMutex.Unlock()
		return tmpl, nil
	}

	if h.defaultTmpl != nil {
		return h.defaultTmpl, nil
	}

	return nil, fmt.Errorf("no placeholder template configured")
}
