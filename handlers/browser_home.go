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
	"sifeucr/templates/base"
	homepage "sifeucr/templates/home"
	"sifeucr/templates/meta"
)

func (h *Handler) browserPage(w http.ResponseWriter, r *http.Request, status int, title string, content templ.Component, reactive bool) {
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
	if err := base.Page(base.PageParams{Nonce: nonce, Links: h.links, Head: base.HeadParams{Title: meta.AppTitle, Subtitle: title}, Body: base.BodyParams{Content: content}}).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render browser page", "error", err)
	}
}

func (h *Handler) browserHome(w http.ResponseWriter, r *http.Request, cookies browser.Cookies, status int, notice string, verifyEmail bool) {
	login, home := h.links.Mount("/login"), h.links.Mount("/home")
	fragment := r.URL.Query().Get("fragment") == "sessions"
	view := homepage.View{VerifyEmail: verifyEmail, Notice: notice, Failed: status >= 400}
	// Verify once, then reuse the request identity for profile and session reads.
	protected := middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, _ := middleware.AuthenticatedIdentity(r.Context())
		if !fragment {
			account, ok := h.loadAccount(w, r, identity.Sub)
			if !ok {
				return
			}
			view.Email = account.Email
		}
		h.ListSessions(w, r)
	}))
	reply := browser.Call(r, protected, nil, cookies.Read(r, "access"))
	if reply.Status == http.StatusUnauthorized {
		w.Header().Set("Cache-Control", "no-store")
		browserRedirect(w, r, login)
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
	rows := make([]homepage.Session, 0, len(sessions.Sessions))
	for _, session := range sessions.Sessions {
		browserName, osName := deviceNames(session.UserAgent)
		rows = append(rows, homepage.Session{ID: session.ID, Device: session.UserAgent, Browser: browserName, OS: osName, Created: session.CreatedAt, Used: session.LastUsedAt, Expires: session.ExpiresAt, Current: session.Current})
	}
	next := ""
	if sessions.NextCursor != "" {
		next = home + "?cursor=" + url.QueryEscape(sessions.NextCursor)
	}
	tr := func(key string) string { return h.localize(r, key) }
	if fragment {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("Vary", "Accept-Language")
		if err := homepage.Sessions(login, home, next, rows, tr, h.language(r)).Render(r.Context(), w); err != nil {
			h.logger.Error("failed to render browser sessions", "error", err)
		}
		return
	}
	h.browserPage(w, r, status, tr("web.home"), homepage.Home(login, home, next, rows, h.language(r), view, tr), true)
}

// Browser forms adapt existing JSON handlers, retaining live permission checks.
func (h *Handler) browserEmailChange(w http.ResponseWriter, r *http.Request, cookies browser.Cookies) {
	confirm := r.PostForm.Get("action") == "email_confirm"
	var payload any = changeEmailRequest{NewEmail: r.PostForm.Get("new_email")}
	next := http.HandlerFunc(h.RequestEmailChange)
	fallback := "err.save_email_change"
	if confirm {
		payload = changeEmailConfirmRequest{OTP: r.PostForm.Get("otp")}
		next, fallback = h.ConfirmEmailChange, "err.confirm_email_change"
	}
	authenticated := false
	var authenticatedRequest *http.Request
	protected := middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated = true
		authenticatedRequest = r
		h.requireEmailChangePermission(next).ServeHTTP(w, r)
	}))
	reply := browser.Call(r, protected, payload, cookies.Read(r, "access"))
	if !authenticated && reply.Status == http.StatusUnauthorized {
		browserRedirect(w, r, h.links.Mount("/login"))
		return
	}
	if browserJSON(r) {
		if reply.Status >= 400 {
			browserForward(w, reply)
			return
		}
		state := map[string]any{"emailVerify": !confirm, "error": "", "notice": ""}
		if confirm {
			profile := browser.Call(authenticatedRequest, http.HandlerFunc(h.GetAccount), nil, cookies.Read(r, "access"))
			if profile.Status != 200 {
				browserForward(w, profile)
				return
			}
			var account accountResponse
			if err := json.Unmarshal(profile.Body.Bytes(), &account); err != nil {
				h.writeError(
					w,
					r,
					http.StatusInternalServerError,
					err,
					"web.update_error",
					"failed to decode updated browser account",
				)
				return
			}
			state["email"] = account.Email
			state["notice"] = h.localize(r, "web.email_updated")
		}
		httpx.WriteJSON(w, 200, state)
		return
	}
	if reply.Status >= 400 {
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(reply.Body.Bytes(), &failure) != nil || failure.Error == "" {
			failure.Error = h.localize(r, fallback)
		}
		h.browserHome(w, r, cookies, reply.Status, failure.Error, confirm)
		return
	}
	step := "verify"
	if confirm {
		step = "updated"
	}
	http.Redirect(w, r, h.links.Mount("/home")+"?email="+step, http.StatusSeeOther)
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
