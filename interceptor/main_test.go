package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/tracing"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

const falseStr = "false"

func TestRunProxyServerCountMiddleware(t *testing.T) {
	const (
		port = 8080
		host = "samplehost"
	)
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	originSrv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer originSrv.Close()
	originPort, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	g, ctx := errgroup.WithContext(ctx)
	q := queue.NewFakeCounter()

	httpso := targetFromURL(
		originURL,
		originPort,
		"testdepl",
		"testservice",
	)
	namespacedName := k8s.NamespacedNameFromObject(httpso).String()

	// set up a fake host that we can spoof
	// when we later send request to the proxy,
	// so that the proxy calculates a URL for that
	// host that points to the (above) fake origin
	// server
	routingTable := routingtest.NewTable()
	routingTable.Memory[host] = httpso
	svcCache := k8s.NewFakeServiceCache()

	timeouts := &config.Timeouts{}
	waiterCh := make(chan struct{})
	waitFunc := func(_ context.Context, _, _ string) (bool, error) {
		<-waiterCh
		return false, nil
	}

	tracingCfg := config.Tracing{Enabled: true, Exporter: "otlphttp"}

	_, err = tracing.SetupOTelSDK(ctx, &tracingCfg)

	if err != nil {
		fmt.Println(err, "Error setting up tracer")
	}

	g.Go(func() error {
		return runProxyServer(
			ctx,
			logr.Discard(),
			q,
			waitFunc,
			routingTable,
			svcCache,
			timeouts,
			&config.Serving{EnableColdStartHeader: true},
			port,
			false,
			map[string]interface{}{},
			&tracingCfg,
		)
	})
	// wait for server to start
	time.Sleep(500 * time.Millisecond)

	// make an HTTP request in the background
	g.Go(func() error {
		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"http://0.0.0.0:%d", port,
			), nil,
		)
		if err != nil {
			return err
		}
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"unexpected status code: %d",
				resp.StatusCode,
			)
		}
		if _, ok := resp.Header["Traceparent"]; !ok {
			return fmt.Errorf("expected Traceparent header to exist, but the header wasn't found")
		}

		if resp.Header.Get("X-KEDA-HTTP-Cold-Start") != "false" {
			return fmt.Errorf("expected X-KEDA-HTTP-Cold-Start false, but got %s", resp.Header.Get("X-KEDA-HTTP-Cold-Start"))
		}
		return nil
	})
	time.Sleep(100 * time.Millisecond)
	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(500 * time.Millisecond):
		r.Fail("timeout waiting for +1 queue resize")
	}

	// tell the wait func to proceed
	select {
	case waiterCh <- struct{}{}:
	case <-time.After(5 * time.Second):
		r.Fail("timeout producing on waiterCh")
	}

	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(5 * time.Second):
		r.Fail("timeout waiting for -1 queue resize")
	}

	// check the queue to make sure all counts are at 0
	countsPtr, err := q.Current()
	r.NoError(err)
	counts := countsPtr.Counts
	r.Equal(1, len(counts))
	_, foundHost := counts[namespacedName]
	r.True(
		foundHost,
		"couldn't find host %s in the queue",
		host,
	)
	r.Equal(0, counts[namespacedName].Concurrency)

	done()
	r.Error(g.Wait())
}

func TestRunProxyServerWithTLSCountMiddleware(t *testing.T) {
	const (
		port = 8443
		host = "samplehost"
	)
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	originSrv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer originSrv.Close()
	originPort, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	g, ctx := errgroup.WithContext(ctx)
	q := queue.NewFakeCounter()

	httpso := targetFromURL(
		originURL,
		originPort,
		"testdepl",
		"testsvc",
	)
	namespacedName := k8s.NamespacedNameFromObject(httpso).String()

	// set up a fake host that we can spoof
	// when we later send request to the proxy,
	// so that the proxy calculates a URL for that
	// host that points to the (above) fake origin
	// server
	routingTable := routingtest.NewTable()
	routingTable.Memory[host] = httpso
	svcCache := k8s.NewFakeServiceCache()

	timeouts := &config.Timeouts{}
	waiterCh := make(chan struct{})
	waitFunc := func(_ context.Context, _, _ string) (bool, error) {
		<-waiterCh
		return false, nil
	}
	tracingCfg := config.Tracing{Enabled: true, Exporter: "otlphttp"}

	g.Go(func() error {
		return runProxyServer(
			ctx,
			logr.Discard(),
			q,
			waitFunc,
			routingTable,
			svcCache,
			timeouts,
			&config.Serving{EnableColdStartHeader: true},
			port,
			true,
			map[string]interface{}{"certificatePath": "../certs/tls.crt", "keyPath": "../certs/tls.key", "skipVerify": true},
			&tracingCfg,
		)
	})

	// wait for server to start
	time.Sleep(500 * time.Millisecond)

	// make an HTTPs request in the background
	g.Go(func() error {
		f, err := os.ReadFile("../certs/RootCA.pem")
		if err != nil {
			t.Errorf("Unable to find RootCA for test, please run tests via `make test`")
		}
		rootCAs, _ := x509.SystemCertPool()
		rootCAs.AppendCertsFromPEM(f)

		http.DefaultClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}

		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"https://localhost:%d", port,
			), nil,
		)
		if err != nil {
			return err
		}
		req.Host = host
		// Allow us to use our self made certs
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"unexpected status code: %d",
				resp.StatusCode,
			)
		}
		if resp.Header.Get("X-KEDA-HTTP-Cold-Start") != falseStr {
			return fmt.Errorf("expected X-KEDA-HTTP-Cold-Start false, but got %s", resp.Header.Get("X-KEDA-HTTP-Cold-Start"))
		}
		return nil
	})
	time.Sleep(100 * time.Millisecond)
	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(2000 * time.Millisecond):
		r.Fail("timeout waiting for +1 queue resize")
	}

	// tell the wait func to proceed
	select {
	case waiterCh <- struct{}{}:
	case <-time.After(5 * time.Second):
		r.Fail("timeout producing on waiterCh")
	}

	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(5 * time.Second):
		r.Fail("timeout waiting for -1 queue resize")
	}

	// check the queue to make sure all counts are at 0
	countsPtr, err := q.Current()
	r.NoError(err)
	counts := countsPtr.Counts
	r.Equal(1, len(counts))
	_, foundHost := counts[namespacedName]
	r.True(
		foundHost,
		"couldn't find host %s in the queue",
		host,
	)
	r.Equal(0, counts[namespacedName].Concurrency)

	done()
	r.Error(g.Wait())
}

func TestRunProxyServerWithMultipleCertsTLSCountMiddleware(t *testing.T) {
	const (
		port = 8443
		host = "samplehost"
	)
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	originSrv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer originSrv.Close()
	originPort, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	g, ctx := errgroup.WithContext(ctx)
	q := queue.NewFakeCounter()

	httpso := targetFromURL(
		originURL,
		originPort,
		"testdepl",
		"testsvc",
	)
	namespacedName := k8s.NamespacedNameFromObject(httpso).String()

	// set up a fake host that we can spoof
	// when we later send request to the proxy,
	// so that the proxy calculates a URL for that
	// host that points to the (above) fake origin
	// server
	routingTable := routingtest.NewTable()
	routingTable.Memory[host] = httpso
	svcCache := k8s.NewFakeServiceCache()

	timeouts := &config.Timeouts{}
	waiterCh := make(chan struct{})
	waitFunc := func(_ context.Context, _, _ string) (bool, error) {
		<-waiterCh
		return false, nil
	}

	tracingCfg := config.Tracing{Enabled: true, Exporter: "otlphttp"}

	g.Go(func() error {
		return runProxyServer(
			ctx,
			logr.Discard(),
			q,
			waitFunc,
			routingTable,
			svcCache,
			timeouts,
			&config.Serving{EnableColdStartHeader: true},
			port,
			true,
			map[string]interface{}{"certstorePaths": "../certs"},
			&tracingCfg,
		)
	})

	// wait for server to start
	time.Sleep(500 * time.Millisecond)

	// make an HTTPs request in the background
	g.Go(func() error {
		f, err := os.ReadFile("../certs/RootCA.pem")
		if err != nil {
			t.Errorf("Unable to find RootCA for test, please run tests via `make test`")
		}
		rootCAs, _ := x509.SystemCertPool()
		rootCAs.AppendCertsFromPEM(f)

		http.DefaultClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}

		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"https://localhost:%d", port,
			), nil,
		)
		if err != nil {
			return err
		}
		req.Host = host
		// Allow us to use our self made certs
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"unexpected status code: %d",
				resp.StatusCode,
			)
		}
		if resp.Header.Get("X-KEDA-HTTP-Cold-Start") != falseStr {
			return fmt.Errorf("expected X-KEDA-HTTP-Cold-Start false, but got %s", resp.Header.Get("X-KEDA-HTTP-Cold-Start"))
		}
		return nil
	})
	time.Sleep(100 * time.Millisecond)
	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(2000 * time.Millisecond):
		r.Fail("timeout waiting for +1 queue resize")
	}

	// tell the wait func to proceed
	select {
	case waiterCh <- struct{}{}:
	case <-time.After(5 * time.Second):
		r.Fail("timeout producing on waiterCh")
	}

	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(namespacedName, hostAndCount.Host)
		r.Equal(1, hostAndCount.Count)
	case <-time.After(5 * time.Second):
		r.Fail("timeout waiting for -1 queue resize")
	}

	// check the queue to make sure all counts are at 0
	countsPtr, err := q.Current()
	r.NoError(err)
	counts := countsPtr.Counts
	r.Equal(1, len(counts))
	_, foundHost := counts[namespacedName]
	r.True(
		foundHost,
		"couldn't find host %s in the queue",
		host,
	)
	r.Equal(0, counts[namespacedName].Concurrency)

	done()
	r.Error(g.Wait())
}
