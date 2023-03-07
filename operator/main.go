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
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/kedacore/http-add-on/pkg/build"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
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

	interceptorCfg, err := config.NewInterceptorFromEnv()
	if err != nil {
		setupLog.Error(err, "unable to get interceptor configuration")
		os.Exit(1)
	}
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

	routingTable := routing.NewTable()
	if err = (&httpcontrollers.HTTPScaledObjectReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		InterceptorConfig:    *interceptorCfg,
		ExternalScalerConfig: *externalScalerCfg,
		BaseConfig:           *baseConfig,
		RoutingTable:         routingTable,
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

	// TODO(pedrotorres): uncomment after implementing new routing table
	// setupLog.Info("starting manager")
	// if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
	// 	setupLog.Error(err, "problem running manager")
	// 	os.Exit(1)
	// }

	// TODO(pedrotorres): remove everything beyond this line after implementing
	// new routing table
	ctx := ctrl.SetupSignalHandler()
	if err := ensureConfigMap(
		ctx,
		setupLog,
		baseConfig.CurrentNamespace,
		routing.ConfigMapRoutingTableName,
	); err != nil {
		setupLog.Error(
			err,
			"unable to find routing table ConfigMap",
			"namespace",
			baseConfig.CurrentNamespace,
			"name",
			routing.ConfigMapRoutingTableName,
		)
		os.Exit(1)
	}

	errGrp, ctx := errgroup.WithContext(ctx)
	ctx, done := context.WithCancel(ctx)

	// start the control loop
	errGrp.Go(func() error {
		defer done()
		setupLog.Info("starting manager")
		return mgr.Start(ctx)
	})

	// start the admin server to serve routing table information
	// to the interceptors
	errGrp.Go(func() error {
		defer done()
		return runAdminServer(
			ctx,
			ctrl.Log,
			routingTable,
			adminPort,
			baseConfig,
			interceptorCfg,
			externalScalerCfg,
		)
	})
	build.PrintComponentInfo(setupLog, "Operator")
	setupLog.Error(errGrp.Wait(), "running the operator")
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	routingTable *routing.Table,
	port int,
	baseCfg *config.Base,
	interceptorCfg *config.Interceptor,
	externalScalerCfg *config.ExternalScaler,

) error {
	mux := http.NewServeMux()
	routing.AddFetchRoute(setupLog, mux, routingTable)
	kedahttp.AddConfigEndpoint(
		lggr.WithName("operatorAdmin"),
		mux,
		baseCfg,
		interceptorCfg,
		externalScalerCfg,
	)
	kedahttp.AddVersionEndpoint(lggr.WithName("operatorAdmin"), mux)
	addr := fmt.Sprintf(":%d", port)
	lggr.Info(
		"starting admin RPC server",
		"port",
		port,
	)
	return kedahttp.ServeContext(ctx, addr, mux)
}

// ensureConfigMap returns a non-nil error if the config
// map in the given namespace with the given name
// does not exist, or there was an error finding it.
//
// it returns a nil error if it could be fetched.
// this function works with its own Kubernetes client and
// is intended for use on operator startup, and should
// not be used with the controller library's client,
// since that is not usable until after the controller
// has started up.
func ensureConfigMap(
	ctx context.Context,
	lggr logr.Logger,
	ns,
	name string,
) error {
	// we need to get our own Kubernetes clientset
	// here, rather than using the client.Client from
	// the manager because the client will not
	// be instantiated by the time we call this.
	// You need to start the manager before that client
	// is usable.
	clset, _, err := k8s.NewClientset()
	if err != nil {
		lggr.Error(
			err,
			"couldn't get new clientset",
		)
		return err
	}
	if _, err := clset.CoreV1().ConfigMaps(ns).Get(
		ctx,
		name,
		metav1.GetOptions{},
	); err != nil {
		lggr.Error(
			err,
			"couldn't find config map",
			"namespace",
			ns,
			"name",
			name,
		)
		return err
	}
	return nil
}
