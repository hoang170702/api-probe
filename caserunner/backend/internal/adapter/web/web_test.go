package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandlerServesTheAppAndFallsBackToIndex(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"assets/app-1a.js": {Data: []byte("console.log(1)")},
	})
	tests := []struct {
		path, body, cache string
	}{
		{"/", "<html>app</html>", "no-cache"},
		{"/some/client/route", "<html>app</html>", "no-cache"},
		{"/assets/app-1a.js", "console.log(1)", "public, max-age=31536000, immutable"},
		{"/assets/", "<html>app</html>", "no-cache"}, // directories are not listed
	}
	for _, tt := range tests {
		rec := get(h, tt.path)
		if rec.Code != http.StatusOK || rec.Body.String() != tt.body || rec.Header().Get("Cache-Control") != tt.cache {
			t.Errorf("GET %s = %d %q (Cache-Control %q), want 200 %q (%q)",
				tt.path, rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"), tt.body, tt.cache)
		}
	}
}

func TestHandlerExplainsAMissingBuild(t *testing.T) {
	rec := get(handler(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "not built") {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}
