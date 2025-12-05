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
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	httpcontrollers "github.com/kedacore/http-add-on/operator/controllers/http"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
	"github.com/kedacore/http-add-on/pkg/util"
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
// +kubebuilder:rbac:groups=coordination.k8s.io,namespace=keda,resources=leases,verbs=get;list;watch;create;update;patch;delete

func main() {
	var metricsAddr string
	var metricsSecure bool
	var metricsAuth bool
	var metricsCertDir string
	var enableLeaderElection bool
	var probeAddr string
	var profilingAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&metricsSecure, "metrics-secure", false, "Enable secure serving for metrics endpoint.")
	flag.BoolVar(&metricsAuth, "metrics-auth", false, "Enable authentication and authorization for metrics endpoint.")
	flag.StringVar(&metricsCertDir, "metrics-cert-dir", "", "The directory that contains the server certificate and key (tls.crt and tls.key) for the metrics endpoint.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&profilingAddr, "profiling-bind-address", "", "The address the profiling would be exposed on.")
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

	leaseDuration, err := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION")
	if err != nil {
		setupLog.Error(err, "invalid KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION")
		os.Exit(1)
	}

	renewDeadline, err := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE")
	if err != nil {
		setupLog.Error(err, "invalid KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE")
		os.Exit(1)
	}

	retryPeriod, err := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD")
	if err != nil {
		setupLog.Error(err, "invalid KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD")
		os.Exit(1)
	}

	if err := util.ValidateLeaderElectionConfig(leaseDuration, renewDeadline, retryPeriod); err != nil {
		setupLog.Error(err, "invalid leader election configuration")
		os.Exit(1)
	}

	var namespaces map[string]cache.Config
	if baseConfig.WatchNamespace != "" {
		namespaces = map[string]cache.Config{
			baseConfig.WatchNamespace: {},
		}
	}

	// Switch between HTTP (8080) and HTTPS (8443) based on metricsSecure flag
	if metricsSecure {
		metricsAddr = ":8443"
	} else {
		metricsAddr = ":8080"
	}

	metricsOpts := server.Options{
		BindAddress:   metricsAddr,
		SecureServing: metricsSecure,
	}
	if metricsAuth {
		metricsOpts.FilterProvider = filters.WithAuthenticationAndAuthorization
	}
	if metricsCertDir != "" {
		metricsOpts.CertDir = metricsCertDir
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsOpts,
		PprofBindAddress:              profilingAddr,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "http-add-on.keda.sh",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 leaseDuration,
		RenewDeadline:                 renewDeadline,
		RetryPeriod:                   retryPeriod,
		Cache: cache.Options{
			DefaultNamespaces: namespaces,
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
