// Command mockapi is a fake API for trying API Case Runner locally
// (BRD-001 §18). It is a development tool and is not part of the release.
//
//	go run ./cmd/mockapi        # listens on MOCKAPI_BIND, default 127.0.0.1:9999
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	addr := os.Getenv("MOCKAPI_BIND")
	if addr == "" {
		addr = "127.0.0.1:9999"
	}
	slog.Info("mockapi listening", "url", "http://"+addr)
	srv := &http.Server{Addr: addr, Handler: newAPI().handler(), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("mockapi stopped", "err", err)
		os.Exit(1)
	}
}

type api struct {
	mu     sync.Mutex
	hits   map[string]int
	tokens map[string]bool
}

func newAPI() *api { return &api{hits: map[string]int{}, tokens: map[string]bool{}} }

func (a *api) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ok", func(w http.ResponseWriter, _ *http.Request) { reply(w, 200, `{"code":"00","message":"OK"}`) })
	mux.HandleFunc("POST /created", func(w http.ResponseWriter, _ *http.Request) { reply(w, 201, `{"code":"00","message":"Created"}`) })
	mux.HandleFunc("POST /error", func(w http.ResponseWriter, _ *http.Request) { reply(w, 500, `{"code":"ERR_INTERNAL"}`) })
	mux.HandleFunc("POST /redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ok", http.StatusFound) })
	mux.HandleFunc("POST /validate", validate)
	mux.HandleFunc("POST /slow", slow)
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("POST /secure", a.secure)
	mux.HandleFunc("GET /hits", a.listHits)
	mux.HandleFunc("POST /hits/reset", a.resetHits)
	return a.count(mux)
}

// count records one hit per request path, except the /hits endpoints.
func (a *api) count(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/hits") {
			a.mu.Lock()
			a.hits[r.URL.Path]++
			a.mu.Unlock()
		}
		slog.Info("mockapi", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func reply(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// validate answers 400 when data.amount is missing, like an API checking a required field.
func validate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		reply(w, 400, `{"code":"ERR_BAD_JSON"}`)
		return
	}
	if _, ok := body.Data["amount"]; !ok {
		reply(w, 400, `{"code":"ERR_AMOUNT_REQUIRED"}`)
		return
	}
	reply(w, 200, `{"code":"00"}`)
}

// slow waits ?ms= milliseconds (default 1000, at most 60000) before answering.
func slow(w http.ResponseWriter, r *http.Request) {
	ms, err := strconv.Atoi(r.URL.Query().Get("ms"))
	if err != nil || ms < 0 {
		ms = 1000
	}
	select {
	case <-time.After(time.Duration(min(ms, 60000)) * time.Millisecond):
		reply(w, 200, `{"code":"00","message":"slow"}`)
	case <-r.Context().Done():
	}
}

func (a *api) login(w http.ResponseWriter, _ *http.Request) {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	tok := "mock." + hex.EncodeToString(b)
	a.mu.Lock()
	a.tokens[tok] = true
	a.mu.Unlock()
	reply(w, 200, `{"code":"00","data":{"accessToken":"`+tok+`","expiresIn":300}}`)
}

func (a *api) secure(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	a.mu.Lock()
	ok := a.tokens[tok]
	a.mu.Unlock()
	if !ok {
		reply(w, 401, `{"code":"AUTH_REQUIRED"}`)
		return
	}
	reply(w, 200, `{"code":"00","message":"authorized"}`)
}

func (a *api) listHits(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	b, _ := json.Marshal(a.hits)
	a.mu.Unlock()
	reply(w, 200, string(b))
}

func (a *api) resetHits(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	a.hits = map[string]int{}
	a.mu.Unlock()
	reply(w, 200, `{}`)
}
