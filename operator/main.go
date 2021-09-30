/*


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

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/routing"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = httpv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var adminPort int
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(
		&adminPort,
		"admin-port",
		9090,
		"The port on which to run the admin server. This is the port on which RPCs will be accepted to get the routing table",
	)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "f8508ff1.keda.sh",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

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
	routingTable := routing.NewTable()
	if err := (&controllers.HTTPScaledObjectReconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName("HTTPScaledObject"),
		Scheme:               mgr.GetScheme(),
		InterceptorConfig:    *interceptorCfg,
		ExternalScalerConfig: *externalScalerCfg,
		BaseConfig:           *baseConfig,
		RoutingTable:         routingTable,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HTTPScaledObject")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	ctx := context.Background()
	errGrp, _ := errgroup.WithContext(ctx)

	// start the control loop
	errGrp.Go(func() error {
		setupLog.Info("starting manager")
		return mgr.Start(ctrl.SetupSignalHandler())
	})

	// start the admin server to serve routing table information
	// to the interceptors
	errGrp.Go(func() error {
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
	addr := fmt.Sprintf(":%d", port)
	lggr.Info(
		"starting admin RPC server",
		"port",
		port,
	)
	return kedahttp.ServeContext(ctx, addr, mux)
}
