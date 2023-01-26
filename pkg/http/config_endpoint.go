package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/build"
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

func AddVersionEndpoint(lggr logr.Logger, mux *http.ServeMux) {
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"version": build.Version(),
		}); err != nil {
			lggr.Error(err, "failed to encode version")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	})
}
