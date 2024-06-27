package http

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// SaveStatus will trigger an object update to save the current status
// conditions
func SaveStatus(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpObject client.Object,
) {
	resourceType := httpObject.GetObjectKind().GroupVersionKind().String()
	logger.Info("Updating status", "resource", httpObject.GetName(), "resourceType", resourceType, "resource version", httpObject.GetResourceVersion())

	err := cl.Status().Update(ctx, httpObject)
	if err != nil {
		logger.Error(err, "failed to update status", "object", httpObject)
	} else {
		logger.Info("Updated status", "resource", httpObject.GetName(), "resourceType", resourceType, "resource version", httpObject.GetResourceVersion())
	}
}

// AddOrUpdateCondition adds or update a condition to the HTTPScaledObject
func AddOrUpdateCondition(httpso *httpv1alpha1.HTTPScaledObject, condition httpv1alpha1.HTTPScaledObjectCondition) *httpv1alpha1.HTTPScaledObject {
	found := false
	for i := range httpso.Status.Conditions {
		if httpso.Status.Conditions[i].Reason == condition.Reason {
			found = true
			httpso.Status.Conditions[i] = condition
		}
	}
	if !found {
		httpso.Status.Conditions = append(httpso.Status.Conditions, condition)
	}
	return httpso
}

// CreateCondition initializes a new status condition
func CreateCondition(
	condType httpv1alpha1.HTTPScaledObjectCreationStatus,
	status metav1.ConditionStatus,
	reason httpv1alpha1.HTTPScaledObjectConditionReason,
) *httpv1alpha1.HTTPScaledObjectCondition {
	cond := httpv1alpha1.HTTPScaledObjectCondition{
		Timestamp: time.Now().Format(time.RFC3339),
		Type:      condType,
		Status:    status,
		Reason:    reason,
	}
	return &cond
}

// SetMessage sets the optional reason for the condition
func SetMessage(c *httpv1alpha1.HTTPScaledObjectCondition, message string) *httpv1alpha1.HTTPScaledObjectCondition {
	c.Message = message
	return c
}
