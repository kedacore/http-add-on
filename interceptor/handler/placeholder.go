package handler

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const placeholderScript = `<script>
(function() {
	const checkInterval = {{.RefreshInterval}} * 1000;
	
	async function checkServiceStatus() {
		try {
			const response = await fetch(window.location.href, {
				method: 'HEAD',
				cache: 'no-cache'
			});
			
			const placeholderHeader = response.headers.get('X-KEDA-HTTP-Placeholder-Served');
			
			if (placeholderHeader !== 'true') {
				window.location.reload();
			} else {
				setTimeout(checkServiceStatus, checkInterval);
			}
		} catch (error) {
			console.error('Error checking service status:', error);
			setTimeout(checkServiceStatus, checkInterval);
		}
	}
	
	setTimeout(checkServiceStatus, checkInterval);
})();
</script>`

const defaultPlaceholderTemplateWithoutScript = `<!DOCTYPE html>
<html>
<head>
    <title>Service Starting</title>
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
    </style>
</head>
<body>
    <div class="container">
        <h1>{{.ServiceName}} is starting up...</h1>
        <div class="spinner"></div>
        <p>Please wait while we prepare your service.</p>
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
	templateCache map[string]*cacheEntry
	cacheMutex    sync.RWMutex
	defaultTmpl   *template.Template
	servingCfg    *config.Serving
	enableScript  bool
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
	var defaultTemplate string

	// Try to load template from configured path
	if servingCfg.PlaceholderDefaultTemplatePath != "" {
		content, err := os.ReadFile(servingCfg.PlaceholderDefaultTemplatePath)
		if err == nil {
			defaultTemplate = string(content)
		} else {
			// Fall back to built-in template if file cannot be read
			fmt.Printf("Warning: Could not read placeholder template from %s: %v. Using built-in template.\n",
				servingCfg.PlaceholderDefaultTemplatePath, err)
			defaultTemplate = defaultPlaceholderTemplateWithoutScript
		}
	} else {
		defaultTemplate = defaultPlaceholderTemplateWithoutScript
	}

	// Inject script if enabled
	if servingCfg.PlaceholderEnableScript {
		defaultTemplate = injectPlaceholderScript(defaultTemplate)
	}

	defaultTmpl, err := template.New("default").Parse(defaultTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default template: %w", err)
	}

	return &PlaceholderHandler{
		templateCache: make(map[string]*cacheEntry),
		defaultTmpl:   defaultTmpl,
		servingCfg:    servingCfg,
		enableScript:  servingCfg.PlaceholderEnableScript,
	}, nil
}

// injectPlaceholderScript injects the placeholder refresh script into a template
func injectPlaceholderScript(templateContent string) string {
	lowerContent := strings.ToLower(templateContent)

	// Look for </body> tag (case-insensitive)
	bodyCloseIndex := strings.LastIndex(lowerContent, "</body>")
	if bodyCloseIndex != -1 {
		// Insert script before </body>
		return templateContent[:bodyCloseIndex] + placeholderScript + templateContent[bodyCloseIndex:]
	}

	// Look for </html> tag if no body tag
	htmlCloseIndex := strings.LastIndex(lowerContent, "</html>")
	if htmlCloseIndex != -1 {
		// Insert script before </html>
		return templateContent[:htmlCloseIndex] + placeholderScript + templateContent[htmlCloseIndex:]
	}

	// Check if content appears to be HTML (has any HTML tags)
	if strings.Contains(templateContent, "<") && strings.Contains(templateContent, ">") {
		// It looks like HTML, append the script
		return templateContent + placeholderScript
	}

	// Don't wrap non-HTML content - return as-is
	return templateContent
}

// detectContentType determines the appropriate content type based on Accept header and content
func detectContentType(acceptHeader string, content string) string {
	// Check Accept header for specific content types
	if strings.Contains(acceptHeader, "application/json") {
		return "application/json"
	}
	if strings.Contains(acceptHeader, "application/xml") {
		return "application/xml"
	}
	if strings.Contains(acceptHeader, "text/plain") {
		return "text/plain"
	}

	// Default to HTML for browser requests or when HTML is accepted
	if strings.Contains(acceptHeader, "text/html") || strings.Contains(acceptHeader, "*/*") || acceptHeader == "" {
		// Check if content looks like HTML
		if strings.Contains(content, "<") && strings.Contains(content, ">") {
			return "text/html; charset=utf-8"
		}
	}

	// Try to detect based on content
	trimmed := strings.TrimSpace(content)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		return "application/json"
	}
	if strings.HasPrefix(trimmed, "<") {
		if strings.HasPrefix(trimmed, "<?xml") {
			return "application/xml"
		}
		return "text/html; charset=utf-8"
	}

	return "text/plain; charset=utf-8"
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

	// Detect and set content type based on Accept header and content
	contentType := detectContentType(r.Header.Get("Accept"), content)

	// For non-HTML content, don't inject script even if enabled
	isHTML := strings.Contains(contentType, "text/html")
	if !isHTML && h.enableScript && strings.Contains(content, placeholderScript) {
		// Remove script from non-HTML content
		content = strings.ReplaceAll(content, placeholderScript, "")
	}

	w.Header().Set("Content-Type", contentType)
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
		content := config.Content
		// Only inject script for HTML-like content if enabled
		if h.enableScript && (strings.Contains(content, "<") && strings.Contains(content, ">")) {
			content = injectPlaceholderScript(content)
		}
		tmpl, err := template.New("inline").Parse(content)
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

	return h.defaultTmpl, nil
}
