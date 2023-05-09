// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/build"
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
	externalscaler "github.com/kedacore/http-add-on/proto"
)

// +kubebuilder:rbac:groups="",namespace=keda,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch

func main() {
	lggr, err := pkglog.NewZapr()
	if err != nil {
		log.Fatalf("error creating new logger (%v)", err)
	}

	cfg := mustParseConfig()
	grpcPort := cfg.GRPCPort
	namespace := cfg.TargetNamespace
	svcName := cfg.TargetService
	deplName := cfg.TargetDeployment
	targetPortStr := fmt.Sprintf("%d", cfg.TargetPort)
	targetPendingRequests := cfg.TargetPendingRequests
	targetPendingRequestsInterceptor := cfg.TargetPendingRequestsInterceptor

	k8sCfg, err := ctrl.GetConfig()
	if err != nil {
		lggr.Error(err, "Kubernetes client config not found")
		os.Exit(1)
	}
	k8sCl, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		lggr.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	pinger, err := newQueuePinger(
		context.Background(),
		lggr,
		k8s.EndpointsFuncForK8sClientset(k8sCl),
		namespace,
		svcName,
		deplName,
		targetPortStr,
	)
	if err != nil {
		lggr.Error(err, "creating a queue pinger")
		os.Exit(1)
	}

	// create the deployment informer
	deployInformer := k8s.NewInformerBackedDeploymentCache(
		lggr,
		k8sCl,
		cfg.DeploymentCacheRsyncPeriod,
	)

	httpCl, err := clientset.NewForConfig(k8sCfg)
	if err != nil {
		lggr.Error(err, "creating new HTTP ClientSet")
		os.Exit(1)
	}
	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, cfg.ConfigMapCacheRsyncPeriod)
	httpsoInformer := informershttpv1alpha1.New(sharedInformerFactory, "", nil).HTTPScaledObjects()

	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	grp, ctx := errgroup.WithContext(ctx)

	// start the deployment informer
	grp.Go(func() error {
		defer done()
		return deployInformer.Start(ctx)
	})

	// start the httpso informer
	grp.Go(func() error {
		defer done()
		httpsoInformer.Informer().Run(ctx.Done())
		return ctx.Err()
	})

	grp.Go(func() error {
		defer done()
		return pinger.start(
			ctx,
			time.NewTicker(cfg.QueueTickDuration),
			deployInformer,
		)
	})

	grp.Go(func() error {
		defer done()
		return startGrpcServer(
			ctx,
			lggr,
			grpcPort,
			pinger,
			httpsoInformer,
			int64(targetPendingRequests),
			int64(targetPendingRequestsInterceptor),
		)
	})

	build.PrintComponentInfo(lggr, "Scaler")
	lggr.Error(grp.Wait(), "one or more of the servers failed")
}

func startGrpcServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
	httpsoInformer informershttpv1alpha1.HTTPScaledObjectInformer,
	targetPendingRequests int64,
	targetPendingRequestsInterceptor int64,
) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("starting grpc server", "address", addr)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	hs := health.NewServer()
	hs.SetServingStatus("liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	hs.SetServingStatus("readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(
		grpcServer,
		hs,
	)

	externalscaler.RegisterExternalScalerServer(
		grpcServer,
		newImpl(
			lggr,
			pinger,
			httpsoInformer,
			targetPendingRequests,
			targetPendingRequestsInterceptor,
		),
	)

	go func() {
		<-ctx.Done()
		lis.Close()
	}()

	return grpcServer.Serve(lis)
}
