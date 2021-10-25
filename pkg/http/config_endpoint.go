package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
)

func AddConfigEndpoint(lggr logr.Logger, mux *http.ServeMux, configs ...interface{}) {
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"configs": configs,
		}); err != nil {
			lggr.Error(err, "failed to encode configs")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	})
}
