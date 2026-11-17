package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"sifeucr/routes"
	"sifeucr/templates"
)

//go:embed static/*
var staticFS embed.FS

func (h *Handler) registerStaticRoutes() {
	files, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("failed to mount embedded static files")
	}

	prefix, _ := h.URL(routes.Static)
	server := http.StripPrefix(prefix, http.FileServerFS(files))
	h.AddRoute(routes.Static, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == prefix+templates.DatastarScriptPath {
			server.ServeHTTP(immutableAssetWriter{w}, r)
			return
		}
		server.ServeHTTP(w, r)
	}))
	h.AddRoute(routes.APIDocs, server)
}

// Only the checksum-addressed public module is immutable. Error responses and
// application HTML (especially login/home) must not inherit this cache policy.
type immutableAssetWriter struct{ http.ResponseWriter }

func (w immutableAssetWriter) WriteHeader(status int) {
	if status == http.StatusOK || status == http.StatusPartialContent || status == http.StatusNotModified {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.ResponseWriter.WriteHeader(status)
}
