//go:build e2e

package helpers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type PatchDeploymentOption func(ctx context.Context, client klient.Client, dep *appsv1.Deployment) error

// PatchInterceptorDeployment registers setup/teardown on testenv that patches the
// interceptor deployment and restores it to its original state on teardown.
func PatchInterceptorDeployment(testenv env.Environment, opts ...PatchDeploymentOption) {
	PatchDeployment(testenv, AddonNamespace, interceptorDeployment, opts...)
}

// PatchDeployment registers setup/teardown on testenv that patches a deployment
// with the given options and restores it to its original state on teardown.
func PatchDeployment(testenv env.Environment, namespace, name string, opts ...PatchDeploymentOption) {
	var originalSpec *corev1.PodSpec

	testenv.Setup(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		client := cfg.Client()

		original := &appsv1.Deployment{}
		if err := client.Resources().Get(ctx, name, namespace, original); err != nil {
			return ctx, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
		}
		originalSpec = original.Spec.Template.Spec.DeepCopy()

		patched := original.DeepCopy()
		for _, opt := range opts {
			if err := opt(ctx, client, patched); err != nil {
				return ctx, err
			}
		}
		if err := client.Resources().Update(ctx, patched); err != nil {
			return ctx, fmt.Errorf("failed to patch deployment %s/%s: %w", namespace, name, err)
		}

		dep := &appsv1.Deployment{ObjectMeta: patched.ObjectMeta}
		if err := wait.For(
			conditions.New(client.Resources()).ResourceMatch(dep, deploymentRolledOut),
			wait.WithTimeout(defaultWaitTimeout),
		); err != nil {
			return ctx, fmt.Errorf("deployment %s/%s rollout timed out: %w", namespace, name, err)
		}

		return ctx, nil
	})

	testenv.Finish(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if originalSpec == nil {
			return ctx, nil
		}
		client := cfg.Client()
		current := &appsv1.Deployment{}
		if err := client.Resources().Get(ctx, name, namespace, current); err != nil {
			return ctx, fmt.Errorf("failed to get deployment for restore: %w", err)
		}
		current.Spec.Template.Spec = *originalSpec
		if err := client.Resources().Update(ctx, current); err != nil {
			return ctx, fmt.Errorf("failed to restore deployment: %w", err)
		}
		return ctx, nil
	})
}

// WithContainerPort appends a container port to all containers in the deployment.
func WithContainerPort(port int32) PatchDeploymentOption {
	return func(_ context.Context, _ klient.Client, dep *appsv1.Deployment) error {
		for i := range dep.Spec.Template.Spec.Containers {
			dep.Spec.Template.Spec.Containers[i].Ports = append(
				dep.Spec.Template.Spec.Containers[i].Ports,
				corev1.ContainerPort{ContainerPort: port},
			)
		}
		return nil
	}
}

// WithEnvVar appends an environment variable to all containers in the deployment.
func WithEnvVar(name, value string) PatchDeploymentOption {
	return func(_ context.Context, _ klient.Client, dep *appsv1.Deployment) error {
		for i := range dep.Spec.Template.Spec.Containers {
			dep.Spec.Template.Spec.Containers[i].Env = append(
				dep.Spec.Template.Spec.Containers[i].Env,
				corev1.EnvVar{Name: name, Value: value},
			)
		}
		return nil
	}
}

// WithTLSCert creates a certificate for the given DNS names using cert-manager
// and mounts the resulting secret into the deployment at /certs.
func WithTLSCert(dnsNames []string) PatchDeploymentOption {
	return func(ctx context.Context, client klient.Client, dep *appsv1.Deployment) error {
		certName, err := createCertificate(ctx, client, dep.Namespace, caIssuerFromContext(ctx), dnsNames)
		if err != nil {
			return err
		}

		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: tlsCertsVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: certName},
				},
			},
		)
		for i := range dep.Spec.Template.Spec.Containers {
			dep.Spec.Template.Spec.Containers[i].VolumeMounts = append(
				dep.Spec.Template.Spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      tlsCertsVolume,
					MountPath: "/certs",
				},
			)
		}
		return nil
	}
}

// RestartInterceptor triggers a rollout restart of the interceptor deployment
// and waits for the new generation to become fully ready.
func (f *Framework) RestartInterceptor() error {
	f.t.Helper()

	dep := &appsv1.Deployment{}
	if err := f.client.Resources().Get(f.ctx, interceptorDeployment, AddonNamespace, dep); err != nil {
		return fmt.Errorf("getting interceptor deployment: %w", err)
	}

	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string)
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	if err := f.client.Resources().Update(f.ctx, dep); err != nil {
		return fmt.Errorf("updating interceptor deployment: %w", err)
	}

	if err := wait.For(
		conditions.New(f.client.Resources()).ResourceMatch(dep, deploymentRolledOut),
		wait.WithTimeout(defaultWaitTimeout),
	); err != nil {
		return fmt.Errorf("interceptor deployment did not recover: %w", err)
	}
	return nil
}

func deploymentRolledOut(object k8s.Object) bool {
	d, ok := object.(*appsv1.Deployment)
	if !ok {
		return false
	}
	return d.Spec.Replicas != nil &&
		d.Status.ObservedGeneration >= d.Generation &&
		d.Status.UpdatedReplicas == *d.Spec.Replicas &&
		d.Status.ReadyReplicas == *d.Spec.Replicas
}
