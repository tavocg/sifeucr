package handlers

import (
	"net/http"

	"sifeucr/internal/httpx"
	"sifeucr/routes"
	"sifeucr/templates/base"
	"sifeucr/templates/meta"
	"sifeucr/templates/root"
)

func (h *Handler) registerRootRoutes() {
	h.AddRoute(routes.Root, http.HandlerFunc(h.Root))
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept, Accept-Language")
	w.Header().Set("Cache-Control", "no-store")
	contentType := httpx.Negotiate(r, "application/json; charset=utf-8", "text/html; charset=utf-8")
	L := func(key string, args ...any) string {
		return h.localize(r, key, args...)
	}

	if contentType == "text/html; charset=utf-8" {
		documentHeaders(w)
		w.Header().Set("Content-Type", contentType)
		page := base.Page(base.PageParams{
			Links: h.links,
			Head: base.HeadParams{
				Title:       meta.AppTitle,
				Subtitle:    L("root.title"),
				Description: L("root.description"),
				RobotsIndex: true,
			},
			Body: base.BodyParams{
				Content: root.RootMain(h.links, L),
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
