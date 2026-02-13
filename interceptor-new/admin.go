package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

var adminLog = ctrl.Log.WithName("admin")

// AdminServer serves health probes and queue counts on the admin port.
// Traffic here is low-volume (probes + scaler polling), so it uses
// a simple net/http server.
type AdminServer struct {
	config         *Config
	routingTable   *RoutingTable
	queue          *QueueCounter
	transportStats *TransportStats
}

// NewAdminServer creates a new admin server.
func NewAdminServer(
	config *Config,
	rt *RoutingTable,
	q *QueueCounter,
	ts *TransportStats,
) *AdminServer {
	return &AdminServer{
		config:         config,
		routingTable:   rt,
		queue:          q,
		transportStats: ts,
	}
}

// ListenAndServe starts the admin server.
func (as *AdminServer) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", as.handleHealth)
	mux.HandleFunc("/readyz", as.handleHealth)
	mux.HandleFunc("/queue", as.handleQueue)
	mux.HandleFunc("/debug/stats", as.handleDebugStats)

	addr := fmt.Sprintf(":%d", as.config.AdminPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	adminLog.Info("Admin server listening", "addr", addr)
	return server.ListenAndServe()
}

// handleHealth serves both /livez and /readyz.
// Returns 200 once the routing table has synced at least once.
func (as *AdminServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if as.routingTable.HasSynced() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Service Unavailable"))
	}
}

// handleQueue returns the current queue counts (consumed by the Scaler).
func (as *AdminServer) handleQueue(w http.ResponseWriter, _ *http.Request) {
	data, err := as.queue.CurrentJSON()
	if err != nil {
		adminLog.Error(err, "failed to serialize queue counts")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleDebugStats returns diagnostic counters (connection reuse stats, etc.)
func (as *AdminServer) handleDebugStats(w http.ResponseWriter, _ *http.Request) {
	total := as.transportStats.ConnEstablished.Load()
	dnsHits := as.transportStats.DNSCacheHits.Load()
	dnsMisses := as.transportStats.DNSCacheMisses.Load()

	stats := map[string]any{
		"connections_established": total,
		"dns_cache_hits":          dnsHits,
		"dns_cache_misses":        dnsMisses,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
