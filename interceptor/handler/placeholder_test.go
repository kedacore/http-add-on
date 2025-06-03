package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	v1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/routing/test"
)

func TestNewPlaceholderHandler(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()

	handler, err := NewPlaceholderHandler(k8sClient, routingTable)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.k8sClient)
	assert.NotNil(t, handler.routingTable)
	assert.NotNil(t, handler.templateCache)
	assert.NotNil(t, handler.defaultTmpl)
}

func TestServePlaceholder_DisabledConfig(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
	assert.Contains(t, w.Body.String(), "checkServiceStatus")
	assert.Contains(t, w.Body.String(), "checkInterval =  5  * 1000")
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "true", w.Header().Get("X-KEDA-HTTP-Placeholder-Served"))
}

func TestServePlaceholder_InlineContent(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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

	// Verify script was injected
	assert.Contains(t, w.Body.String(), "checkServiceStatus")
	assert.Contains(t, w.Body.String(), "X-KEDA-HTTP-Placeholder-Served")
}

func TestServePlaceholder_ConfigMapContent(t *testing.T) {
	// Create fake k8s client with a ConfigMap
	configMapContent := `<html><body><h1>ConfigMap placeholder for {{.ServiceName}}</h1></body></html>`
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "placeholder-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"template.html": configMapContent,
		},
	}
	k8sClient := fake.NewSimpleClientset(cm)
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
				Enabled:          true,
				StatusCode:       503,
				RefreshInterval:  5,
				ContentConfigMap: "placeholder-cm",
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "ConfigMap placeholder for test-service")

	// Verify script was injected
	assert.Contains(t, w.Body.String(), "checkServiceStatus")
	assert.Contains(t, w.Body.String(), "X-KEDA-HTTP-Placeholder-Served")
}

func TestServePlaceholder_CustomHeaders(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	customContent := `<html><body>{{.ServiceName}}</body></html>`
	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: customContent,
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
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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

func TestGetTemplate_CacheInvalidation_ConfigMapVersion(t *testing.T) {
	// Create ConfigMap
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
}

func TestGetTemplate_ConcurrentAccess(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	customContent := `<html><body>{{.ServiceName}}</body></html>`
	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: customContent,
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
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	customContent := `<html><body>{{.ServiceName}}</body></html>`
	hso := &v1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: customContent,
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
	configMaps := make([]*v1.ConfigMap, 10)
	for i := 0; i < 10; i++ {
		configMaps[i] = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            fmt.Sprintf("placeholder-cm-%d", i),
				Namespace:       "default",
				ResourceVersion: "1",
			},
			Data: map[string]string{
				"template.html": fmt.Sprintf(`<html><body>ConfigMap %d: {{.ServiceName}}</body></html>`, i),
			},
		}
	}

	k8sClient := fake.NewSimpleClientset()
	for _, cm := range configMaps {
		_, err := k8sClient.CoreV1().ConfigMaps("default").Create(context.Background(), cm, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
							ContentConfigMap: fmt.Sprintf("placeholder-cm-%d", cmIndex),
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

func TestInjectPlaceholderScript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTML with body tag",
			input:    `<html><body><h1>Hello</h1></body></html>`,
			expected: `<html><body><h1>Hello</h1>` + placeholderScript + `</body></html>`,
		},
		{
			name:     "HTML with uppercase BODY tag",
			input:    `<HTML><BODY><H1>Hello</H1></BODY></HTML>`,
			expected: `<HTML><BODY><H1>Hello</H1>` + placeholderScript + `</BODY></HTML>`,
		},
		{
			name:     "HTML without body tag",
			input:    `<html><div>Hello</div></html>`,
			expected: `<html><div>Hello</div>` + placeholderScript + `</html>`,
		},
		{
			name:     "HTML fragment with angle brackets",
			input:    `<p>Just some text</p>`,
			expected: `<p>Just some text</p>` + placeholderScript,
		},
		{
			name:  "Empty string",
			input: ``,
			expected: fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Service Starting</title>
</head>
<body>

%s
</body>
</html>`, placeholderScript),
		},
		{
			name:     "Multiple body tags (uses last one)",
			input:    `<body>First</body><body>Second</body>`,
			expected: `<body>First</body><body>Second` + placeholderScript + `</body>`,
		},
		{
			name:     "HTML with only html close tag",
			input:    `<html><div>Hello</div></html>`,
			expected: `<html><div>Hello</div>` + placeholderScript + `</html>`,
		},
		{
			name:  "Non-HTML content gets wrapped",
			input: `Just plain text without HTML`,
			expected: fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Service Starting</title>
</head>
<body>
Just plain text without HTML
%s
</body>
</html>`, placeholderScript),
		},
		{
			name:     "Partial HTML without closing tags",
			input:    `<div>Some content</div>`,
			expected: `<div>Some content</div>` + placeholderScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectPlaceholderScript(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServePlaceholder_InlineContentWithScriptInjection(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	// Custom content without the script
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
				StatusCode:      503,
				RefreshInterval: 10,
				Content:         customContent,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Check that custom content is there
	assert.Contains(t, w.Body.String(), "Custom placeholder for test-service")

	// Check that script was injected
	assert.Contains(t, w.Body.String(), "checkServiceStatus")
	assert.Contains(t, w.Body.String(), "X-KEDA-HTTP-Placeholder-Served")
	assert.Contains(t, w.Body.String(), "checkInterval =  10  * 1000")
}

func TestServePlaceholder_ConfigMapContentWithScriptInjection(t *testing.T) {
	// Create fake k8s client with a ConfigMap
	configMapContent := `<html><body><h1>ConfigMap placeholder for {{.ServiceName}}</h1></body></html>`
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "placeholder-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"template.html": configMapContent,
		},
	}
	k8sClient := fake.NewSimpleClientset(cm)
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

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
				Enabled:          true,
				StatusCode:       503,
				RefreshInterval:  15,
				ContentConfigMap: "placeholder-cm",
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Check that custom content is there
	assert.Contains(t, w.Body.String(), "ConfigMap placeholder for test-service")

	// Check that script was injected
	assert.Contains(t, w.Body.String(), "checkServiceStatus")
	assert.Contains(t, w.Body.String(), "X-KEDA-HTTP-Placeholder-Served")
	assert.Contains(t, w.Body.String(), "checkInterval =  15  * 1000")
}

func TestServePlaceholder_NoBodyTagScriptInjection(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	// Custom content without body tag
	customContent := `<div>Simple placeholder for {{.ServiceName}}</div>`

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
				Content:         customContent,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)

	body := w.Body.String()
	// Check that custom content is there
	assert.Contains(t, body, "Simple placeholder for test-service")

	// Check that script was appended at the end
	assert.True(t, strings.HasSuffix(body, "</script>"))
	assert.Contains(t, body, "checkServiceStatus")
}

func TestServePlaceholder_NonHTMLContentWrapping(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	routingTable := test.NewTable()
	handler, _ := NewPlaceholderHandler(k8sClient, routingTable)

	// Non-HTML content that should get wrapped
	plainTextContent := `Welcome! Your service is starting up.
Please wait a moment...`

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
				Content:         plainTextContent,
			},
		},
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	err := handler.ServePlaceholder(w, req, hso)
	assert.NoError(t, err)

	body := w.Body.String()

	// Check that content was wrapped in proper HTML
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "<html>")
	assert.Contains(t, body, "<body>")
	assert.Contains(t, body, "</body>")
	assert.Contains(t, body, "</html>")

	// Check that original content is preserved
	assert.Contains(t, body, "Welcome! Your service is starting up.")
	assert.Contains(t, body, "Please wait a moment...")

	// Check that script was injected
	assert.Contains(t, body, "checkServiceStatus")
}
