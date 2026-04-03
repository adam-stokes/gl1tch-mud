package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// FS is set by the main package at startup to the embedded web/dist filesystem.
// This indirection is needed because //go:embed paths are relative to the source
// file, and web/dist lives at the repo root, not inside internal/server/.
var FS fs.FS

// FileHandler returns an http.Handler serving the embedded static files.
// HTML files are served with Cache-Control: no-cache so browsers always
// re-fetch index.html and pick up new hashed asset references after a restart.
// Hashed asset files (/_astro/*) keep the default ETag-based caching.
// Panics if FS has not been set.
func FileHandler() http.Handler {
	if FS == nil {
		panic("server.FS not initialized — call server.SetFS before starting")
	}
	base := http.FileServer(http.FS(FS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		base.ServeHTTP(w, r)
	})
}

// SetFS sets the embedded filesystem used to serve static frontend files.
func SetFS(f fs.FS) {
	FS = f
}
