//go:build e2e

package helpers

import (
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

const (
	// renovate: datasource=docker
	defaultImage   = "ghcr.io/traefik/whoami:v1.11.0"
	defaultPort    = int32(8080)
	tlsCertsVolume = "tls-certs"

	ImageGRPCEcho = "grpc-echo"
)

// TestApp bundles a Deployment and Service for a test backend.
type TestApp struct {
	Namespace     string
	Name          string
	Image         string
	Replicas      int32
	Port          int32
	PortName      string // if set, used as the service port name
	TLSSecretName string // if set, mounts this secret and serves TLS
}

// TestAppOption configures a TestApp before its resources are created.
type TestAppOption func(*TestApp)

// AppWithImage overrides the default container image.
func AppWithImage(image string) TestAppOption {
	return func(a *TestApp) { a.Image = image }
}

// AppWithTestImage sets the app's container image to a test image built by
// `make e2e-test-images`. The name must match a directory under test/images/.
func AppWithTestImage(name string) TestAppOption {
	repo := os.Getenv("KO_DOCKER_REPO")
	if repo == "" {
		panic("KO_DOCKER_REPO must be set")
	}
	return AppWithImage(fmt.Sprintf("%s/%s:latest", repo, name))
}

// AppWithReplicas sets the initial replica count for the app's deployment.
func AppWithReplicas(n int32) TestAppOption {
	return func(a *TestApp) { a.Replicas = n }
}

// AppWithPortName sets the service port name.
func AppWithPortName(name string) TestAppOption {
	return func(a *TestApp) { a.PortName = name }
}

// AppWithTLSSecret configures the app to serve TLS using the given cert secret.
func AppWithTLSSecret(secretName string) TestAppOption {
	return func(a *TestApp) { a.TLSSecretName = secretName }
}

// CreateTestApp creates a test backend (Deployment + Service) in the cluster.
func (f *Framework) CreateTestApp(name string, opts ...TestAppOption) *TestApp {
	f.t.Helper()
	app := &TestApp{
		Namespace: f.namespace,
		Name:      name,
		Image:     defaultImage,
		Replicas:  0,
		Port:      defaultPort,
	}
	for _, opt := range opts {
		opt(app)
	}
	for _, obj := range app.Resources() {
		f.createResource(obj)
	}
	return app
}

// Resources returns all Kubernetes objects for this app.
func (a *TestApp) Resources() []k8s.Object {
	var (
		args         = []string{"--verbose"}
		labels       = map[string]string{"app": a.Name}
		probeScheme  = corev1.URISchemeHTTP
		volumeMounts []corev1.VolumeMount
		volumes      []corev1.Volume
	)

	if a.TLSSecretName != "" {
		probeScheme = corev1.URISchemeHTTPS
		args = append(args, "--cert=/certs/tls.crt", "--key=/certs/tls.key")
		volumeMounts = []corev1.VolumeMount{{
			Name:      tlsCertsVolume,
			MountPath: "/certs",
			ReadOnly:  true,
		}}
		volumes = []corev1.Volume{{
			Name: tlsCertsVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: a.TLSSecretName,
				},
			},
		}}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(a.Replicas),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Hostname: a.Name, // override the hostname to allow matching the response to a specific app
					Volumes:  volumes,
					Containers: []corev1.Container{{
						Name:            a.Name,
						Image:           a.Image,
						ImagePullPolicy: a.imagePullPolicy(),
						Args:            args,
						Env: []corev1.EnvVar{
							{
								Name:  "PORT",
								Value: fmt.Sprintf("%d", a.Port),
							},
							{
								Name:  "WHOAMI_PORT_NUMBER",
								Value: fmt.Sprintf("%d", a.Port),
							},
						},
						Ports: []corev1.ContainerPort{
							{ContainerPort: a.Port},
						},
						VolumeMounts:   volumeMounts,
						LivenessProbe:  a.probe(5, probeScheme),
						ReadinessProbe: a.probe(1, probeScheme),
					}},
				},
			},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name,
			Namespace: a.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       a.PortName,
				Port:       a.Port,
				TargetPort: intstr.FromInt32(a.Port),
			}},
		},
	}

	return []k8s.Object{deployment, service}
}

// imagePullPolicy returns IfNotPresent for images loaded into Kind via ko
// (kind.local/), since Kubernetes defaults :latest to Always which fails
// because kind.local is not an actual registry.
func (a *TestApp) imagePullPolicy() corev1.PullPolicy {
	if strings.HasPrefix(a.Image, "kind.local/") {
		return corev1.PullIfNotPresent
	}
	return ""
}

func (a *TestApp) probe(periodSeconds int32, httpScheme corev1.URIScheme) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health",
				Port:   intstr.FromInt32(a.Port),
				Scheme: httpScheme,
			},
		},
		PeriodSeconds:    periodSeconds,
		FailureThreshold: 3,
	}
}
