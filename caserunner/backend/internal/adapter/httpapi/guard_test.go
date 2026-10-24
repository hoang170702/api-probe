package httpapi

import (
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	loopback := Guard(ok, []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"})
	network := Guard(ok, nil)

	tests := []struct {
		name    string
		h       http.Handler
		method  string
		path    string
		host    string
		headers map[string]string
		want    int
	}{
		{"same-origin JSON call", loopback, "POST", "/api/parse", "127.0.0.1:8080",
			map[string]string{"Content-Type": "application/json", "Origin": "http://127.0.0.1:8080", "Sec-Fetch-Site": "same-origin"}, 204},
		{"tool without Origin (curl)", loopback, "POST", "/api/parse", "localhost:8080",
			map[string]string{"Content-Type": "application/json; charset=utf-8"}, 204},
		{"simple cross-site POST (text/plain)", loopback, "POST", "/api/prepare/try", "127.0.0.1:8080",
			map[string]string{"Content-Type": "text/plain"}, 415},
		{"cross-site Origin", loopback, "POST", "/api/prepare/try", "127.0.0.1:8080",
			map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
		{"opaque Origin", loopback, "POST", "/api/prepare/try", "127.0.0.1:8080",
			map[string]string{"Content-Type": "application/json", "Origin": "null"}, 403},
		{"Sec-Fetch-Site cross-site", loopback, "POST", "/api/prepare/try", "127.0.0.1:8080",
			map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}, 403},
		{"DNS rebinding Host", loopback, "GET", "/api/health", "attacker.example:8080", nil, 403},
		{"DNS rebinding Host on the page", loopback, "GET", "/", "attacker.example:8080", nil, 403},
		{"Host check is off on a network bind", network, "GET", "/api/health", "pc-42.corp.local:8080", nil, 204},
		{"CSRF checks stay on a network bind", network, "POST", "/api/prepare/try", "pc-42.corp.local:8080",
			map[string]string{"Content-Type": "text/plain"}, 415},
		{"colleague on a network bind", network, "POST", "/api/parse", "pc-42.corp.local:8080",
			map[string]string{"Content-Type": "application/json", "Origin": "http://pc-42.corp.local:8080"}, 204},
		{"GET needs no Content-Type", loopback, "GET", "/api/health", "127.0.0.1:8080", nil, 204},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
			r.Host = tt.host
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			tt.h.ServeHTTP(rec, r)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestLoopbackHosts(t *testing.T) {
	addr := func(s string) net.Addr { a, _ := net.ResolveTCPAddr("tcp", s); return a }
	if got, want := LoopbackHosts(addr("127.0.0.1:9090")), []string{"127.0.0.1:9090", "localhost:9090", "[::1]:9090"}; !slices.Equal(got, want) {
		t.Errorf("loopback: got %q, want %q", got, want)
	}
	for _, a := range []string{"0.0.0.0:8080", "10.1.2.3:8080", "[::]:8080"} {
		if got := LoopbackHosts(addr(a)); got != nil {
			t.Errorf("%s: got %q, want nil", a, got)
		}
	}
}
