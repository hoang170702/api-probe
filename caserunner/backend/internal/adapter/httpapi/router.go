// Package httpapi is the inbound HTTP adapter: routing, handlers and the
// JSON error format of BRD-001 §15.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// Info describes the running service for the health endpoint.
type Info struct {
	Version string
}

// UseCases are the application rules the handlers call.
type UseCases struct {
	ParseCurl usecase.ParseCurl
}

// NewHandler returns the API handler with logging and panic recovery
// applied. Paths outside /api/ go to app, the frontend, when it is not nil.
func NewHandler(info Info, uc UseCases, app http.Handler) http.Handler {
	routes := []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{http.MethodGet, "/api/health", health(info)},
		{http.MethodPost, "/api/parse", parseCurl(uc.ParseCurl)},
	}
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.HandleFunc(rt.method+" "+rt.path, rt.handler)
	}
	// Unmatched /api/ paths get JSON errors instead of the frontend.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		var allow []string
		for _, rt := range routes {
			if rt.path == r.URL.Path {
				allow = append(allow, rt.method)
			}
		}
		if len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", r.Method+" is not allowed on "+r.URL.Path)
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No API endpoint "+r.Method+" "+r.URL.Path)
	})
	if app != nil {
		mux.Handle("/", app)
	}
	return recoverPanics(logRequests(mux))
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func health(info Info) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "caserunner", Version: info.Version})
	}
}
