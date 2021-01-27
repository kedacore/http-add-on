// This file contains the implementation for the HTTP request queue used by the
// KEDA external scaler implementation
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type queuePinger struct {
	k8sCl        *kubernetes.Clientset
	ns           string
	svcName      string
	adminPort    string
	pingMut      *sync.RWMutex
	lastPingTime time.Time
	lastCount    int
}

func newQueuePinger(
	k8sCl *kubernetes.Clientset,
	ns,
	svcName,
	adminPort string,
	pingTicker *time.Ticker,
) queuePinger {
	pingMut := new(sync.RWMutex)
	pinger := queuePinger{
		k8sCl:     k8sCl,
		ns:        ns,
		svcName:   svcName,
		adminPort: adminPort,
		pingMut:   pingMut,
	}

	go func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-pingTicker.C:
				log.Printf("Tick at %s", time.Now())
				pinger.requestCounts()
			}
		}

	}()

	return pinger
}

func (q queuePinger) count() int {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.lastCount
}

func (q queuePinger) requestCounts() error {
	endpointsCl := q.k8sCl.CoreV1().Endpoints(q.ns)
	endpoints, err := endpointsCl.Get(q.svcName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	queueSizeCh := make(chan int)
	var wg sync.WaitGroup

	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			wg.Add(1)
			go func(addr string) {
				defer wg.Done()
				completeAddr := fmt.Sprintf("http://%s:%s/queue", addr, q.adminPort)
				resp, err := http.Get(completeAddr)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				respData := map[string]int{}
				if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
					return
				}
				curSize := respData["current_size"]
				log.Printf("Current size for address %s: %d", addr, curSize)
				queueSizeCh <- curSize
			}(addr.Hostname)
		}
	}

	go func() {
		wg.Wait()
		close(queueSizeCh)
	}()

	total := 0
	for count := range queueSizeCh {
		total += count
	}

	q.pingMut.Lock()
	defer q.pingMut.Unlock()
	q.lastCount = total
	q.lastPingTime = time.Now()

	return nil

}
