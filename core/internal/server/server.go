// Package server exposes Core's two HTTP surfaces: the session-authenticated
// management API and the mTLS-authenticated Agent API. Phase 0 implements
// health and version reporting only; product routes arrive in later phases.
package server

import (
	"encoding/json"
	"net/http"

	"autosecrets.dev/core/internal/config"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// New builds the Core HTTP handler.
func New(cfg config.Config, version string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET "+cfg.ManagementBase+"/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "core", Version: version})
	})

	agentHealth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "core-agent", Version: version})
	})
	mux.Handle("GET "+cfg.AgentBase+"/health", AgentIdentityMiddleware(cfg.TrustedProxyCIDRs, cfg.ProxyCertHeader)(agentHealth))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	return mux
}
