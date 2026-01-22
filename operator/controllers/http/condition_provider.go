package http

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// MigrateConditions filters out old conditions that have zero LastTransitionTime.
// This is needed because old conditions used a "timestamp" field which doesn't map to
// metav1.Condition and contained duplicate Ready conditions resulting in errors.
//
// TODO(v1): Remove this migration helper when graduating to v1.
func MigrateConditions(conditions []metav1.Condition) []metav1.Condition {
	var migrated []metav1.Condition
	for _, c := range conditions {
		if !c.LastTransitionTime.IsZero() {
			migrated = append(migrated, c)
		}
	}
	return migrated
}

// SaveStatus persists the current status to the API server.
func SaveStatus(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpso *httpv1alpha1.HTTPScaledObject,
) error {
	if err := cl.Status().Update(ctx, httpso); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	logger.Info("Updated HTTPScaledObject status", "resourceVersion", httpso.ResourceVersion)
	return nil
}
