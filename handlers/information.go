package handlers

import (
	"net/http"
	"sifeucr/routes"
	"sifeucr/templates/information"
)

func (h *Handler) registerInformationRoutes() {
	h.AddRoute(routes.AboutPage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr := func(key string) string { return h.localize(r, key) }
		h.browserPage(w, r, http.StatusOK, tr("web.about"), information.About(h.links, tr), false)
	}))
	h.AddRoute(routes.TermsPage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr := func(key string) string { return h.localize(r, key) }
		h.browserPage(w, r, http.StatusOK, tr("web.terms"), information.Terms(tr), false)
	}))
}
