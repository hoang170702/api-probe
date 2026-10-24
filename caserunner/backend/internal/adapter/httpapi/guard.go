package httpapi

import (
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// Guard rejects requests another web site makes through the tester's
// browser (BRD-001 §13):
//   - a state-changing /api/ call must be JSON, so a browser always sends a
//     CORS preflight first, which this server never grants;
//   - a request whose Origin is another site is refused;
//   - when hosts is not empty, a Host header outside it is refused, which
//     stops DNS rebinding (a foreign domain pointed at 127.0.0.1).
func Guard(next http.Handler, hosts []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(hosts) > 0 && !slices.Contains(hosts, strings.ToLower(r.Host)) {
			reject(w, r, http.StatusForbidden, "FORBIDDEN", "Host "+r.Host+" is not accepted; open the tool at http://"+hosts[0])
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
			reject(w, r, http.StatusForbidden, "FORBIDDEN", "Cross-site request refused")
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			reject(w, r, http.StatusForbidden, "FORBIDDEN", "Cross-site request refused")
			return
		}
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			reject(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Send the request body as application/json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, host)
}

// reject logs the refusal without headers, which may carry tokens.
func reject(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	slog.Warn("request refused", "method", r.Method, "path", r.URL.Path, "status", status, "reason", msg)
	writeError(w, status, code, msg)
}

// LoopbackHosts returns the Host values a browser sends for a server
// listening on addr when addr is a loopback address, or nil otherwise: a
// server reachable from the network is opened by names this one cannot know.
func LoopbackHosts(addr net.Addr) []string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil
	}
	return []string{"127.0.0.1:" + port, "localhost:" + port, "[::1]:" + port}
}
