//go:build e2e

package helpers

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// GenerateLoad sends sustained HTTP traffic through the interceptor proxy
// at the given rate for the specified duration. It returns a stop function
// that halts the attack, waits for all results to be collected, and returns
// the aggregated metrics.
func (f *Framework) GenerateLoad(targetURL url.URL, pacer vegeta.Pacer, duration time.Duration) func() vegeta.Metrics {
	f.t.Helper()

	host := targetURL.Host
	targetURL.Scheme = "http"
	targetURL.Host = f.proxyAddr

	target := vegeta.Target{
		Method: http.MethodGet,
		URL:    targetURL.String(),
		Header: http.Header{"Host": []string{host}},
	}
	targeter := vegeta.NewStaticTargeter(target)

	attacker := vegeta.NewAttacker()
	resultCh := attacker.Attack(targeter, pacer, duration, host)

	var wg sync.WaitGroup
	var metrics vegeta.Metrics
	wg.Go(func() {
		for res := range resultCh {
			metrics.Add(res)
		}
		metrics.Close()
	})

	stop := func() vegeta.Metrics {
		attacker.Stop()
		wg.Wait()
		return metrics
	}

	// Ensure vegeta always stops early even when tests fail
	f.t.Cleanup(func() { stop() })

	return stop
}
