package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const testCustomContent = `<html><body>{{.ServiceName}}</body></html>`

func TestNewPlaceholderHandler(t *testing.T) {
	servingCfg := &config.Serving{}

	handler, err := NewPlaceholderHandler(servingCfg)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.servingCfg)
	assert.NotNil(t, handler.templateCache)
}

func TestServePlaceholder_DisabledConfig(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	// Create HTTPScaledObject with disabled placeholder
	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled: false,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Service temporarily unavailable")
}

func TestServePlaceholder_DefaultTemplate(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	// Create HTTPScaledObject with enabled placeholder
	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "test-service is starting up...")
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "true", w.Header().Get("X-KEDA-HTTP-Placeholder-Served"))
}

func TestServePlaceholder_InlineContent(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	customContent := `<html><body><h1>Custom placeholder for {{.ServiceName}}</h1></body></html>`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      202,
				RefreshInterval: 3,
				Content:         customContent,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "Custom placeholder for test-service")
}

func TestServePlaceholder_NonHTMLContent(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	// Test with JSON content
	jsonContent := `{"status": "starting", "service": "{{.ServiceName}}"}`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
				Content:         jsonContent,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"service": "test-service"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServePlaceholder_CustomHeaders(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"X-Service-Name":  "test-service",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "test-service", w.Header().Get("X-Service-Name"))
}

func TestServePlaceholder_InvalidTemplate(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	// Invalid template syntax
	invalidContent := `<html><body>{{.UnknownField</body></html>`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "test-service",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
				Content:         invalidContent,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err) // Should not return error, but fall back
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	// Should fall back to simple response
	assert.Contains(t, w.Body.String(), "test-service is starting up...")
}

func TestGetTemplate_Caching(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: testCustomContent,
			},
		},
	}

	ctx := context.Background()

	tmpl1, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl1)

	tmpl2, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl2)

	assert.Equal(t, fmt.Sprintf("%p", tmpl1), fmt.Sprintf("%p", tmpl2))
}

func TestGetTemplate_CacheInvalidation_Generation(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	customContent1 := `<html><body>Version 1: {{.ServiceName}}</body></html>`
	customContent2 := `<html><body>Version 2: {{.ServiceName}}</body></html>`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: customContent1,
			},
		},
	}

	ctx := context.Background()

	tmpl1, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl1)

	hso.Generation = 2
	hso.Spec.PlaceholderConfig.Content = customContent2

	tmpl2, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl2)

	assert.NotEqual(t, fmt.Sprintf("%p", tmpl1), fmt.Sprintf("%p", tmpl2))
}

func TestGetTemplate_CacheInvalidation_ConfigMapVersion_REMOVED(t *testing.T) {
	t.Skip("ConfigMap support removed per maintainer feedback")
	return
	// The code below is kept for reference but won't be executed
	/*
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "placeholder-cm",
				Namespace:       "default",
				ResourceVersion: "1",
			},
			Data: map[string]string{
				"template.html": `<html><body>Version 1: {{.ServiceName}}</body></html>`,
			},
		}
		k8sClient := fake.NewSimpleClientset(cm)
		routingTable := test.NewTable()
		handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

		hso := &v1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-app",
				Namespace:  "default",
				Generation: 1,
			},
			Spec: v1alpha1.HTTPScaledObjectSpec{
				PlaceholderConfig: &v1alpha1.PlaceholderConfig{
					ContentConfigMap: "placeholder-cm",
				},
			},
		}

		ctx := context.Background()

		tmpl1, err := handler.getTemplate(ctx, hso)
		require.NoError(t, err)
		assert.NotNil(t, tmpl1)

		cm.ResourceVersion = "2"
		cm.Data["template.html"] = `<html><body>Version 2: {{.ServiceName}}</body></html>`
		_, err = k8sClient.CoreV1().ConfigMaps("default").Update(ctx, cm, metav1.UpdateOptions{})
		require.NoError(t, err)

		tmpl2, err := handler.getTemplate(ctx, hso)
		require.NoError(t, err)
		assert.NotNil(t, tmpl2)
	*/
}

func TestGetTemplate_ConcurrentAccess(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: testCustomContent,
			},
		},
	}

	ctx := context.Background()

	_, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	templates := make(chan interface{}, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tmpl, err := handler.getTemplate(ctx, hso)
			if err != nil {
				errors <- err
			} else {
				templates <- tmpl
			}
		}()
	}

	wg.Wait()
	close(errors)
	close(templates)

	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
		errorCount++
	}
	assert.Equal(t, 0, errorCount)

	var firstTemplate interface{}
	templateCount := 0
	for tmpl := range templates {
		templateCount++
		if firstTemplate == nil {
			firstTemplate = tmpl
		} else {
			assert.Equal(t, fmt.Sprintf("%p", firstTemplate), fmt.Sprintf("%p", tmpl),
				"All templates should be the same cached instance")
		}
	}
	assert.Equal(t, 100, templateCount)
}

func TestGetTemplate_ConcurrentFirstAccess(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: testCustomContent,
			},
		},
	}

	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := handler.getTemplate(ctx, hso)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	handler.cacheMutex.RLock()
	cacheKey := fmt.Sprintf("%s/%s/inline", hso.Namespace, hso.Name)
	entry, ok := handler.templateCache[cacheKey]
	handler.cacheMutex.RUnlock()

	assert.True(t, ok, "Cache should have an entry")
	assert.NotNil(t, entry, "Cache entry should not be nil")
	assert.NotNil(t, entry.template, "Cached template should not be nil")
}

func TestGetTemplate_ConcurrentCacheUpdates(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func(cmIndex, iteration int) {
				defer wg.Done()

				hso := &v1alpha1.HTTPScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:       fmt.Sprintf("test-app-%d", cmIndex),
						Namespace:  "default",
						Generation: int64(iteration),
					},
					Spec: v1alpha1.HTTPScaledObjectSpec{
						PlaceholderConfig: &v1alpha1.PlaceholderConfig{
							Content: fmt.Sprintf(`<html><body>Template %d-%d: {{.ServiceName}}</body></html>`, cmIndex, iteration),
						},
					},
				}

				_, err := handler.getTemplate(ctx, hso)
				if err != nil {
					errors <- err
				}
			}(i, j)
		}
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent cache update error: %v", err)
	}
}

// Content-Agnostic Tests - Verify the feature works with any content format

func TestServePlaceholder_JSONResponse(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	jsonContent := `{
  "status": "warming_up",
  "service": "{{.ServiceName}}",
  "namespace": "{{.Namespace}}",
  "retry_after_seconds": {{.RefreshInterval}},
  "timestamp": "{{.Timestamp}}"
}`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-service",
			Namespace: "production",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "api-backend",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      202,
				RefreshInterval: 10,
				Content:         jsonContent,
				Headers: map[string]string{
					"Content-Type": "application/json",
					"Retry-After":  "10",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://api.example.com/users", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "10", w.Header().Get("Retry-After"))

	body := w.Body.String()
	assert.Contains(t, body, `"service": "api-backend"`)
	assert.Contains(t, body, `"namespace": "production"`)
	assert.Contains(t, body, `"retry_after_seconds": 10`)
	assert.Contains(t, body, `"status": "warming_up"`)
}

func TestServePlaceholder_XMLResponse(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<response>
  <status>unavailable</status>
  <service>{{.ServiceName}}</service>
  <namespace>{{.Namespace}}</namespace>
  <message>Service is scaling up</message>
  <retryAfter>{{.RefreshInterval}}</retryAfter>
</response>`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-service",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "legacy-backend",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
				Content:         xmlContent,
				Headers: map[string]string{
					"Content-Type": "application/xml; charset=utf-8",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://legacy.example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/xml; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	// Note: html/template escapes XML, so < becomes &lt;
	assert.Contains(t, body, "legacy-backend")
	assert.Contains(t, body, "default")
	assert.Contains(t, body, "<retryAfter>5</retryAfter>")
	assert.Contains(t, body, "xml version")
}

func TestServePlaceholder_PlainTextResponse(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	textContent := `{{.ServiceName}} is currently unavailable.

The service is scaling up to handle your request.
Please retry in {{.RefreshInterval}} seconds.

Namespace: {{.Namespace}}
Request ID: {{.RequestID}}`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-service",
			Namespace: "apps",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "simple-backend",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 3,
				Content:         textContent,
				Headers: map[string]string{
					"Content-Type": "text/plain; charset=utf-8",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://simple.example.com", nil)
	req.Header.Set("X-Request-ID", "abc-123-xyz")
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "simple-backend is currently unavailable")
	assert.Contains(t, body, "Please retry in 3 seconds")
	assert.Contains(t, body, "Namespace: apps")
	assert.Contains(t, body, "Request ID: abc-123-xyz")
}

func TestServePlaceholder_HTMLWithUserControlledRefresh(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	htmlContent := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="refresh" content="{{.RefreshInterval}}">
  <title>{{.ServiceName}} - Starting Up</title>
</head>
<body>
  <h1>{{.ServiceName}} is starting...</h1>
  <p>The service will be ready soon. This page will refresh automatically.</p>
  <p>Namespace: {{.Namespace}}</p>
</body>
</html>`

	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-app",
			Namespace: "frontend",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: v1alpha1.ScaleTargetRef{
				Service: "web-frontend",
			},
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Enabled:         true,
				StatusCode:      503,
				RefreshInterval: 5,
				Content:         htmlContent,
				Headers: map[string]string{
					"Content-Type": "text/html; charset=utf-8",
				},
			},
		},
	}

	req := httptest.NewRequest("GET", "http://webapp.example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, `<meta http-equiv="refresh" content="5">`)
	assert.Contains(t, body, "<h1>web-frontend is starting...</h1>")
	assert.Contains(t, body, "Namespace: frontend")
	// Verify no automatic script injection
	assert.NotContains(t, body, "checkServiceStatus")
}

func TestServePlaceholder_ContentTypeUserControl(t *testing.T) {
	servingCfg := &config.Serving{}
	handler, _ := NewPlaceholderHandler(servingCfg)

	// Test that user-provided Content-Type is respected
	// Note: Currently using html/template which auto-escapes, so XML/HTML chars become entities
	testCases := []struct {
		name            string
		content         string
		contentType     string
		expectedContent string
	}{
		{
			name:            "application/json",
			content:         `{"service": "{{.ServiceName}}"}`,
			contentType:     "application/json",
			expectedContent: `test`,
		},
		{
			name:            "text/plain",
			content:         `Service: {{.ServiceName}}`,
			contentType:     "text/plain",
			expectedContent: `Service: test`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hso := &v1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:       fmt.Sprintf("test-app-%d", i),
					Namespace:  "default",
					Generation: int64(i + 1),
				},
				Spec: v1alpha1.HTTPScaledObjectSpec{
					ScaleTargetRef: v1alpha1.ScaleTargetRef{
						Service: "test",
					},
					PlaceholderConfig: &v1alpha1.PlaceholderConfig{
						Enabled: true,
						Content: tc.content,
						Headers: map[string]string{
							"Content-Type": tc.contentType,
						},
					},
				},
			}

			req := httptest.NewRequest("GET", "http://example.com", nil)
			w := httptest.NewRecorder()

			err := handler.ServePlaceholder(w, req, hso)
			assert.NoError(t, err)
			assert.Equal(t, tc.contentType, w.Header().Get("Content-Type"))
			assert.Contains(t, w.Body.String(), tc.expectedContent)
		})
	}
}
