/*
Copyright 2023 The KEDA Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	httpcontrollers "github.com/kedacore/http-add-on/operator/controllers/http"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(httpv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// +kubebuilder:rbac:groups="",namespace=keda,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",namespace=keda,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,namespace=keda,resources=leases,verbs=get;list;watch;create;update;patch;delete

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var adminPort int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	// TODO(pedrotorres): remove after implementing new routing table
	flag.IntVar(
		&adminPort,
		"admin-port",
		9090,
		"The port on which to run the admin server. This is the port on which RPCs will be accepted to get the routing table",
	)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	externalScalerCfg, err := config.NewExternalScalerFromEnv()
	if err != nil {
		setupLog.Error(err, "unable to get external scaler configuration")
		os.Exit(1)
	}
	baseConfig, err := config.NewBaseFromEnv()
	if err != nil {
		setupLog.Error(
			err,
			"unable to get base configuration",
		)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f8508ff1.keda.sh",
		// TODO(pedrotorres): uncomment after implementing new routing table
		// LeaderElectionReleaseOnCancel: true,
		Namespace: baseConfig.WatchNamespace,

		// TODO(pedrotorres): remove this when we stop relying on ConfigMaps for the routing table
		// workaround for using the same K8s client for both the routing table and the HTTPScaledObject
		// this was already broken if the operator was running only for a single namespace
		ClientDisableCacheFor: []client.Object{
			&corev1.ConfigMap{},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&httpcontrollers.HTTPScaledObjectReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		ExternalScalerConfig: *externalScalerCfg,
		BaseConfig:           *baseConfig,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HTTPScaledObject")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
