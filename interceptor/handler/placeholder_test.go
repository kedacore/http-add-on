package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	assert.Contains(t, w.Body.String(), "refresh")
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
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: v1alpha1.HTTPScaledObjectSpec{
			PlaceholderConfig: &v1alpha1.PlaceholderConfig{
				Content: customContent,
			},
		},
	}

	ctx := context.Background()

	// First call - should parse and cache
	tmpl1, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl1)

	// Second call - should return from cache
	tmpl2, err := handler.getTemplate(ctx, hso)
	require.NoError(t, err)
	assert.NotNil(t, tmpl2)

	// Should be the same template instance
	assert.Equal(t, fmt.Sprintf("%p", tmpl1), fmt.Sprintf("%p", tmpl2))
}
