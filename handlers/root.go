package handlers

import (
	"net/http"

	"sifeucr/internal/httpx"
	"sifeucr/templates/base"
	"sifeucr/templates/meta"
	"sifeucr/templates/root"
)

func (h *Handler) registerRootRoutes() {
	route := "/"
	if h.rootPrefix == "" {
		route = "/{$}"
	}
	h.Add(http.MethodGet, route, h.Root)
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept, Accept-Language")
	w.Header().Set("Cache-Control", "no-store")
	contentType := httpx.Negotiate(r, "application/json; charset=utf-8", "text/html; charset=utf-8")
	L := func(key string, args ...any) string {
		return h.localize(r, key, args...)
	}

	if contentType == "text/html; charset=utf-8" {
		w.Header().Set("Content-Type", contentType)
		page := base.Page(base.PageParams{
			Head: base.HeadParams{
				Title:       meta.AppTitle,
				Subtitle:    L("root.greeting"),
				RobotsIndex: true,
			},
			Body: base.BodyParams{
				Content: root.RootMain(L),
			},
		})

		if err := page.Render(r.Context(), w); err != nil {
			h.writeError(
				w,
				r,
				http.StatusInternalServerError,
				err,
				"err.render_root",
				"failed to render root page",
			)
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": L("root.greeting"),
	})
}
