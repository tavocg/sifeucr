package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"sifeucr/internal/browser"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	"sifeucr/routes"
	"sifeucr/templates/base"
	"sifeucr/templates/meta"
	mypage "sifeucr/templates/my"
)

func (h *Handler) browserPage(w http.ResponseWriter, r *http.Request, status int, title string, content templ.Component, reactive bool, aside ...templ.Component) {
	var navigation templ.Component
	if len(aside) > 0 {
		navigation = aside[0]
	}
	documentHeaders(w)
	nonce := ""
	if reactive {
		var entropy [32]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			h.writeError(
				w,
				r,
				http.StatusInternalServerError,
				err,
				"web.update_error",
				"failed to generate document nonce",
			)
			return
		}
		nonce = base64.StdEncoding.EncodeToString(entropy[:])
		w.Header().Set("Content-Security-Policy", strings.Replace(documentCSP, "script-src 'self'", "script-src 'self' 'nonce-"+nonce+"'", 1))
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Vary", "Accept-Language")
	w.WriteHeader(status)
	if err := base.Page(base.PageParams{Tr: func(key string) string { return h.localize(r, key) }, Nonce: nonce, Links: h.links, Head: base.HeadParams{Title: meta.AppTitle, Subtitle: title}, Body: base.BodyParams{Content: content, Aside: navigation, MyArea: navigation != nil}}).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render browser page", "error", err)
	}
}

func (h *Handler) browserMyPage(w http.ResponseWriter, r *http.Request, cookies browser.Cookies, status int) {
	loginURL, _ := h.URL(routes.LoginPage)
	myURL, _ := h.URL(routes.MyPage)
	fragment := r.URL.Query().Get("fragment") == "sessions"
	view := mypage.View{}
	// Verify once, then reuse the request identity for profile and session reads.
	protected := middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, _ := middleware.AuthenticatedIdentity(r.Context())
		if !fragment {
			account, ok := h.loadAccount(w, r, identity.Sub)
			if !ok {
				return
			}
			view.Email = account.Email
			view.Created = account.CreatedAt
		}
		h.ListSessions(w, r)
	}))
	reply := browser.Call(r, protected, nil, cookies.Read(r, "access"))
	if reply.Status == http.StatusUnauthorized {
		w.Header().Set("Cache-Control", "no-store")
		browserRedirect(w, r, loginURL)
		return
	}
	if reply.Status != http.StatusOK {
		browserForward(w, reply)
		return
	}
	var sessions accountSessionsResponse
	if err := json.Unmarshal(reply.Body.Bytes(), &sessions); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.select_sessions",
			"failed to decode browser sessions",
		)
		return
	}
	rows := make([]mypage.Session, 0, len(sessions.Sessions))
	for _, session := range sessions.Sessions {
		browserName, osName := deviceNames(session.UserAgent)
		rows = append(rows, mypage.Session{ID: session.ID, Device: session.UserAgent, Browser: browserName, OS: osName, Created: session.CreatedAt, Used: session.LastUsedAt, Expires: session.ExpiresAt, Current: session.Current})
	}
	nextURL := ""
	if sessions.NextCursor != "" {
		nextURL = myURL + "?cursor=" + url.QueryEscape(sessions.NextCursor)
	}
	tr := func(key string) string { return h.localize(r, key) }
	if fragment {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("Vary", "Accept-Language")
		if err := mypage.Sessions(loginURL, myURL, nextURL, rows, tr, h.language(r)).Render(r.Context(), w); err != nil {
			h.logger.Error("failed to render browser sessions", "error", err)
		}
		return
	}
	h.browserPage(w, r, status, tr("web.home"), mypage.Page(loginURL, myURL, nextURL, rows, h.language(r), view, tr), true, mypage.Aside(view, tr))
}

// Only the browser adapter negotiates JSON; the public API remains Bearer-only.
func browserJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

func browserRedirect(w http.ResponseWriter, r *http.Request, target string) {
	if browserJSON(r) {
		httpx.WriteJSON(w, 200, map[string]string{"redirect": target})
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func browserForward(w http.ResponseWriter, reply *browser.Reply) {
	for key, values := range reply.HeaderMap {
		w.Header()[key] = values
	}
	w.WriteHeader(reply.Status)
	_, _ = w.Write(reply.Body.Bytes())
}
