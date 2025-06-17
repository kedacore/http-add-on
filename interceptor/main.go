package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	k8sinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/handler"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	"github.com/kedacore/http-add-on/pkg/build"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch

func main() {
	defer os.Exit(1)
	timeoutCfg := config.MustParseTimeouts()
	servingCfg := config.MustParseServing()
	metricsCfg := config.MustParseMetrics()
	tracingCfg := config.MustParseTracing()

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if err := config.Validate(servingCfg, *timeoutCfg, ctrl.Log); err != nil {
		setupLog.Error(err, "invalid configuration")
		runtime.Goexit()
	}

	setupLog.Info(
		"starting interceptor",
		"timeoutConfig",
		timeoutCfg,
		"servingConfig",
		servingCfg,
		"metricsConfig",
		metricsCfg,
	)

	proxyPort := servingCfg.ProxyPort
	adminPort := servingCfg.AdminPort
	proxyTLSEnabled := servingCfg.ProxyTLSEnabled
	profilingAddr := servingCfg.ProfilingAddr

	// setup the configured metrics collectors
	metrics.NewMetricsCollectors(metricsCfg)

	cfg := ctrl.GetConfigOrDie()

	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "creating new Kubernetes ClientSet")
		runtime.Goexit()
	}

	k8sSharedInformerFactory := k8sinformers.NewSharedInformerFactory(cl, time.Millisecond*time.Duration(servingCfg.EndpointsCachePollIntervalMS))
	svcCache := k8s.NewInformerBackedServiceCache(ctrl.Log, cl, k8sSharedInformerFactory)
	endpointsCache := k8s.NewInformerBackedEndpointsCache(ctrl.Log, cl, time.Millisecond*time.Duration(servingCfg.EndpointsCachePollIntervalMS))
	if err != nil {
		setupLog.Error(err, "creating new endpoints cache")
		runtime.Goexit()
	}
	waitFunc := newWorkloadReplicasForwardWaitFunc(ctrl.Log, endpointsCache)

	httpCl, err := clientset.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "creating new HTTP ClientSet")
		runtime.Goexit()
	}

	queues := queue.NewMemory()

	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, servingCfg.ConfigMapCacheRsyncPeriod)
	routingTable, err := routing.NewTable(sharedInformerFactory, servingCfg.WatchNamespace, queues)
	if err != nil {
		setupLog.Error(err, "fetching routing table")
		runtime.Goexit()
	}

	setupLog.Info("Interceptor starting")

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, ctrl.Log)

	eg, ctx := errgroup.WithContext(ctx)

	if tracingCfg.Enabled {
		shutdown, err := tracing.SetupOTelSDK(ctx, tracingCfg)

		if err != nil {
			setupLog.Error(err, "Error setting up tracer")
		}

		defer func() {
			err = errors.Join(err, shutdown(context.Background()))
		}()
	}

	// start the endpoints cache updater
	eg.Go(func() error {
		setupLog.Info("starting the endpoints cache")

		endpointsCache.Start(ctx)
		k8sSharedInformerFactory.Start(ctx.Done())
		return nil
	})

	// start the update loop that updates the routing table from
	// the ConfigMap that the operator updates as HTTPScaledObjects
	// enter and exit the system
	eg.Go(func() error {
		setupLog.Info("starting the routing table")

		if err := routingTable.Start(ctx); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "routing table failed")
			return err
		}

		return nil
	})

	// start the administrative server. this is the server
	// that serves the queue size API
	eg.Go(func() error {
		setupLog.Info("starting the admin server", "port", adminPort)

		if err := runAdminServer(ctx, ctrl.Log, queues, adminPort); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "admin server failed")
			return err
		}

		return nil
	})

	if metricsCfg.OtelPrometheusExporterEnabled {
		// start the prometheus compatible metrics server
		// serves a prometheus compatible metrics endpoint on the configured port
		eg.Go(func() error {
			if err := runMetricsServer(ctx, ctrl.Log, metricsCfg); !util.IsIgnoredErr(err) {
				setupLog.Error(err, "could not start the Prometheus metrics server")
				return err
			}

			return nil
		})
	}

	// start the proxy servers. This is the server that
	// accepts, holds and forwards user requests
	// start a proxy server with TLS
	if proxyTLSEnabled {
		eg.Go(func() error {
			proxyTLSConfig := map[string]string{"certificatePath": servingCfg.TLSCertPath, "keyPath": servingCfg.TLSKeyPath, "certstorePaths": servingCfg.TLSCertStorePaths}
			proxyTLSPort := servingCfg.TLSPort
			k8sSharedInformerFactory.WaitForCacheSync(ctx.Done())

			setupLog.Info("starting the proxy server with TLS enabled", "port", proxyTLSPort)

			if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, svcCache, timeoutCfg, proxyTLSPort, proxyTLSEnabled, proxyTLSConfig, tracingCfg); !util.IsIgnoredErr(err) {
				setupLog.Error(err, "tls proxy server failed")
				return err
			}
			return nil
		})
	}

	// start a proxy server without TLS.
	eg.Go(func() error {
		k8sSharedInformerFactory.WaitForCacheSync(ctx.Done())
		setupLog.Info("starting the proxy server with TLS disabled", "port", proxyPort)

		k8sSharedInformerFactory.WaitForCacheSync(ctx.Done())
		if err := runProxyServer(ctx, ctrl.Log, queues, waitFunc, routingTable, svcCache, timeoutCfg, proxyPort, false, nil, tracingCfg); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "proxy server failed")
			return err
		}

		return nil
	})

	if len(profilingAddr) > 0 {
		eg.Go(func() error {
			setupLog.Info("enabling pprof for profiling", "address", profilingAddr)
			return http.ListenAndServe(profilingAddr, nil)
		})
	}

	build.PrintComponentInfo(ctrl.Log, "Interceptor")

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		setupLog.Error(err, "fatal error")
		runtime.Goexit()
	}

	setupLog.Info("Bye!")
}

func runAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	q queue.Counter,
	port int,
) error {
	lggr = lggr.WithName("runAdminServer")
	adminServer := http.NewServeMux()
	queue.AddCountsRoute(
		lggr,
		adminServer,
		q,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("admin server starting", "address", addr)
	return kedahttp.ServeContext(ctx, addr, adminServer, nil)
}

func runMetricsServer(
	ctx context.Context,
	lggr logr.Logger,
	metricsCfg *config.Metrics,
) error {
	lggr.Info("starting the prometheus metrics server", "port", metricsCfg.OtelPrometheusExporterPort, "path", "/metrics")
	addr := fmt.Sprintf("0.0.0.0:%d", metricsCfg.OtelPrometheusExporterPort)
	return kedahttp.ServeContext(ctx, addr, promhttp.Handler(), nil)
}

// addCert adds a certificate to the map of certificates based on the certificate's SANs
func addCert(m map[string]tls.Certificate, certPath, keyPath string, logger logr.Logger) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("error loading certificate and key: %w", err)
	}
	if cert.Leaf == nil {
		if len(cert.Certificate) == 0 {
			return nil, fmt.Errorf("no certificate found in certificate chain")
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("error parsing certificate: %w", err)
		}
	}
	for _, d := range cert.Leaf.DNSNames {
		logger.Info("adding certificate", "dns", d)
		m[d] = cert
	}
	for _, ip := range cert.Leaf.IPAddresses {
		logger.Info("adding certificate", "ip", ip.String())
		m[ip.String()] = cert
	}
	for _, uri := range cert.Leaf.URIs {
		logger.Info("adding certificate", "uri", uri.String())
		m[uri.String()] = cert
	}
	return &cert, nil
}

func defaultCertPool(logger logr.Logger) *x509.CertPool {
	systemCAs, err := x509.SystemCertPool()
	if err == nil {
		return systemCAs
	}

	logger.Info("error loading system CA pool, using empty pool", "error", err)
	return x509.NewCertPool()
}

// getTLSConfig creates a TLS config from KEDA_HTTP_PROXY_TLS_CERT_PATH, KEDA_HTTP_PROXY_TLS_KEY_PATH and KEDA_HTTP_PROXY_TLS_CERTSTORE_PATHS
// The matching between request and certificate is performed by comparing TLS/SNI server name with x509 SANs
func getTLSConfig(tlsConfig map[string]string, logger logr.Logger) (*tls.Config, error) {
	certPath := tlsConfig["certificatePath"]
	keyPath := tlsConfig["keyPath"]
	certStorePaths := tlsConfig["certstorePaths"]
	servingTLS := &tls.Config{
		RootCAs: defaultCertPool(logger),
	}
	var defaultCert *tls.Certificate

	uriDomainsToCerts := make(map[string]tls.Certificate)
	if certPath != "" && keyPath != "" {
		cert, err := addCert(uriDomainsToCerts, certPath, keyPath, logger)
		if err != nil {
			return servingTLS, fmt.Errorf("error adding certificate and key: %w", err)
		}
		defaultCert = cert
		rawCert, err := os.ReadFile(certPath)
		if err != nil {
			return servingTLS, fmt.Errorf("error reading certificate: %w", err)
		}
		servingTLS.RootCAs.AppendCertsFromPEM(rawCert)
	}

	if certStorePaths != "" {
		certFiles := make(map[string]string)
		keyFiles := make(map[string]string)
		dirPaths := strings.Split(certStorePaths, ",")
		for _, dir := range dirPaths {
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				switch {
				case strings.HasSuffix(path, "-key.pem"):
					certID := path[:len(path)-8]
					keyFiles[certID] = path
				case strings.HasSuffix(path, ".pem"):
					certID := path[:len(path)-4]
					certFiles[certID] = path
				case strings.HasSuffix(path, ".key"):
					certID := path[:len(path)-4]
					keyFiles[certID] = path
				case strings.HasSuffix(path, ".crt"):
					certID := path[:len(path)-4]
					certFiles[certID] = path
				}
				return nil
			})
			if err != nil {
				return servingTLS, fmt.Errorf("error walking certificate store: %w", err)
			}
		}

		for certID, certPath := range certFiles {
			logger.Info("adding certificate", "certID", certID, "certPath", certPath)
			keyPath, ok := keyFiles[certID]
			if !ok {
				return servingTLS, fmt.Errorf("no key found for certificate %s", certPath)
			}
			if _, err := addCert(uriDomainsToCerts, certPath, keyPath, logger); err != nil {
				return servingTLS, fmt.Errorf("error adding certificate %s: %w", certPath, err)
			}
			rawCert, err := os.ReadFile(certPath)
			if err != nil {
				return servingTLS, fmt.Errorf("error reading certificate: %w", err)
			}
			servingTLS.RootCAs.AppendCertsFromPEM(rawCert)
		}
	}

	servingTLS.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cert, ok := uriDomainsToCerts[hello.ServerName]; ok {
			return &cert, nil
		}
		if defaultCert != nil {
			return defaultCert, nil
		}
		return nil, fmt.Errorf("no certificate found for %s", hello.ServerName)
	}
	servingTLS.Certificates = maps.Values(uriDomainsToCerts)
	return servingTLS, nil
}

func runProxyServer(
	ctx context.Context,
	logger logr.Logger,
	q queue.Counter,
	waitFunc forwardWaitFunc,
	routingTable routing.Table,
	svcCache k8s.ServiceCache,
	timeouts *config.Timeouts,
	port int,
	tlsEnabled bool,
	tlsConfig map[string]string,
	tracingConfig *config.Tracing,
) error {
	dialer := kedanet.NewNetDialer(timeouts.Connect, timeouts.KeepAlive)
	dialContextFunc := kedanet.DialContextWithRetry(dialer, timeouts.DefaultBackoff())

	probeHandler := handler.NewProbe([]util.HealthChecker{
		routingTable,
	})
	go probeHandler.Start(ctx)

	var tlsCfg *tls.Config
	if tlsEnabled {
		cfg, err := getTLSConfig(tlsConfig, logger)
		if err != nil {
			logger.Error(fmt.Errorf("error creating certGetter for proxy server"), "error", err)
			os.Exit(1)
		}
		tlsCfg = cfg
	}

	var upstreamHandler http.Handler
	forwardingTLSCfg := &tls.Config{}
	if tlsCfg != nil {
		forwardingTLSCfg.RootCAs = tlsCfg.RootCAs
		forwardingTLSCfg.Certificates = tlsCfg.Certificates
	}

	upstreamHandler = newForwardingHandler(
		logger,
		dialContextFunc,
		waitFunc,
		newForwardingConfigFromTimeouts(timeouts),
		forwardingTLSCfg,
		tracingConfig,
	)
	upstreamHandler = middleware.NewCountingMiddleware(
		q,
		upstreamHandler,
	)

	var rootHandler http.Handler
	rootHandler = middleware.NewRouting(
		routingTable,
		probeHandler,
		upstreamHandler,
		svcCache,
		tlsEnabled,
	)

	if tracingConfig.Enabled {
		rootHandler = otelhttp.NewHandler(rootHandler, "keda-http-interceptor")
	}

	rootHandler = middleware.NewLogging(
		logger,
		rootHandler,
	)

	rootHandler = middleware.NewMetrics(
		rootHandler,
	)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	logger.Info("proxy server starting", "address", addr)
	if tlsEnabled {
		return kedahttp.ServeContext(ctx, addr, rootHandler, tlsCfg)
	}
	return kedahttp.ServeContext(ctx, addr, rootHandler, nil)
}
