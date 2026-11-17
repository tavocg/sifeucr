package handlers

import (
	"fmt"
	"io/fs"
	"net/http"

	"sifeucr/routes"
	"sifeucr/templates"
)

// AddRoute registers a named route. Complete registration before serving requests.
func (h *Handler) AddRoute(id routes.ID, handler http.Handler, options ...RouteOptions) {
	d, err := id.Definition()
	if err != nil {
		panic(err)
	}
	if h.namedRoutes[id] {
		panic(fmt.Sprintf("route %d registered twice", id))
	}
	path := d.Path
	if id == routes.Root && h.links.Mount("/") == "/" {
		path = "/{$}"
	}
	h.AddHandler(d.Method, path, handler, options...)
	h.namedRoutes[id] = true
}

func (h *Handler) URL(id routes.ID, values ...string) (string, error) {
	return h.links.URL(id, values...)
}

func (h *Handler) validateRoutes() error {
	for _, id := range routes.All() {
		if !h.namedRoutes[id] {
			return fmt.Errorf("named route %d is not registered", id)
		}
	}
	// These linked assets must exist in the binary, not merely match the static route.
	for _, name := range []string{"static/api/index.html", "static/" + templates.SemanticStylesheetPath, "static/" + templates.DatastarScriptPath} {
		info, err := fs.Stat(staticFS, name)
		if err != nil {
			return fmt.Errorf("linked asset %s: %w", name, err)
		}
		if info.IsDir() {
			return fmt.Errorf("linked asset %s is a directory", name)
		}
	}
	return nil
}
