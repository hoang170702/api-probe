// Package web serves the frontend build embedded in the binary, so one
// executable runs the whole tool. scripts/build.sh copies the Vite build
// into dist/ before compiling.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

const notBuilt = `The frontend is not built into this binary.
Build it with caserunner/scripts/build.sh, or run the Vite dev server (cd caserunner/frontend && npm run dev).
`

// Handler serves the embedded single-page app: existing files as is, any
// other path as index.html.
func Handler() http.Handler {
	return handler(mustSub(dist, "dist"))
}

func handler(files fs.FS) http.Handler {
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(notBuilt))
		})
	}
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "index.html" || !exists(files, name) {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		if strings.HasPrefix(name, "assets/") { // Vite puts a content hash in these names
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func exists(files fs.FS, name string) bool {
	info, err := fs.Stat(files, name)
	return err == nil && !info.IsDir()
}

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err) // dir is a constant that go:embed guarantees exists
	}
	return sub
}
