package server

import (
	"io/fs"
	"net/http"
)

// FS is set by the main package at startup to the embedded web/dist filesystem.
// This indirection is needed because //go:embed paths are relative to the source
// file, and web/dist lives at the repo root, not inside internal/server/.
var FS fs.FS

// FileHandler returns an http.Handler serving the embedded static files.
// Panics if FS has not been set.
func FileHandler() http.Handler {
	if FS == nil {
		panic("server.FS not initialized — call server.SetFS before starting")
	}
	return http.FileServer(http.FS(FS))
}

// SetFS sets the embedded filesystem used to serve static frontend files.
func SetFS(f fs.FS) {
	FS = f
}
