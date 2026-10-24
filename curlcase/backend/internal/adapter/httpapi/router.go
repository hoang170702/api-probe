// Package httpapi is the inbound HTTP adapter: routing, handlers and the
// JSON error format of BRD-001 §15.
package httpapi

import "net/http"

// Info describes the running service for the health endpoint.
type Info struct {
	Version string
}

// NewHandler returns the API handler with logging and panic recovery applied.
func NewHandler(info Info) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health(info))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No API endpoint "+r.Method+" "+r.URL.Path)
	})
	return recoverPanics(logRequests(mux))
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func health(info Info) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "curlcase", Version: info.Version})
	}
}
