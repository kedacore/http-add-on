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
) *queuePinger {
	pingMut := new(sync.RWMutex)
	pinger := &queuePinger{
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
				if err := pinger.requestCounts(); err != nil {
					log.Printf("Error getting request counts (%s)", err)
				}
			}
		}

	}()

	return pinger
}

func (q *queuePinger) count() int {
	q.pingMut.RLock()
	defer q.pingMut.RUnlock()
	return q.lastCount
}

func (q *queuePinger) requestCounts() error {
	log.Printf("queuePinger.requestCounts")
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
					log.Printf("Error in pinger with GET %s (%s)", completeAddr, err)
					return
				}
				defer resp.Body.Close()
				respData := map[string]int{}
				if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
					log.Printf("Error decoding request to %s (%s)", completeAddr, err)
					return
				}
				curSize := respData["current_size"]
				log.Printf("\n--\ncurSize for address %s: %d\n--\n", addr, curSize)
				queueSizeCh <- curSize
				log.Printf("Sent curSize %d for address %s", curSize, addr)
			}(addr.IP)
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
	log.Printf("Finished getting aggregate current size %d", q.lastCount)

	return nil

}
