package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/mssola/useragent"
	goauth "github.com/tavocg/go-auth"
	"sifeucr/internal/browser"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	"sifeucr/routes"
	loginpage "sifeucr/templates/login"
)

// Remove this file and its registration call, plus routes.LoginPage, to remove
// the browser surface. API handlers, JSON contracts and Bearer authentication are
// not modified.
func (h *Handler) registerBrowserRoutes() {
	const loginPath = "/login"
	cookies := browser.Cookies{Path: h.links.Mount("/"), Secure: !h.dev}
	if !strings.HasSuffix(cookies.Path, "/") {
		cookies.Path += "/"
	}
	loginURL := h.links.Mount(loginPath)
	myURL, _ := h.URL(routes.MyPage)
	page := h.browserPage
	showLogin := func(w http.ResponseWriter, r *http.Request, status int, email, message string) {
		tr := func(key string) string { return h.localize(r, key) }
		if browserJSON(r) && status >= 400 {
			httpx.WriteJSONError(w, status, message)
			return
		}
		resume := status == http.StatusOK && r.Method == http.MethodGet && cookies.Read(r, "refresh") != "" && r.URL.Query().Get("resume") != "0" && !prefetch(r)
		page(w, r, status, tr("web.login"), loginpage.Login(loginURL, email, message, resume, tr), resume)
	}

	// GET never consumes a rotating token; the browser coordinates an automatic
	// POST between tabs. A valid access cookie can redirect immediately.
	h.AddRoute(routes.LoginPage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated, err := h.browserAuthenticated(r, cookies)
		if err != nil {
			h.logger.Error("failed to verify browser session", "error", err)
			status, _ := httpx.ErrorStatus(r.Context(), http.StatusServiceUnavailable, "err.verify_bearer")
			showLogin(w, r, status, "", h.localize(r, "err.verify_bearer"))
			return
		}
		if authenticated {
			w.Header().Set("Cache-Control", "no-store")
			browserRedirect(w, r, myURL)
			return
		}
		showLogin(w, r, http.StatusOK, "", "")
	}))

	// Production cross-origin form posts are rejected. Debug logs record the
	// compared headers, because tunnels and proxies can rewrite Host.
	csrf := http.NewCrossOriginProtection()
	if h.dev {
		csrf.AddInsecureBypassPattern("POST " + loginURL)
	}
	for _, origin := range h.csrfOrigins {
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			h.logger.Error("failed to trust browser form origin", "origin", origin, "error", err)
		}
	}
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.writeStatus(
			w,
			r,
			http.StatusForbidden,
			"err.permission_denied",
			"cross-origin browser form rejected",
			"reason",
			csrf.Check(r),
			"origin",
			r.Header.Get("Origin"),
			"host",
			r.Host,
			"sec_fetch_site",
			r.Header.Get("Sec-Fetch-Site"),
		)
	}))
	h.AddHandler("POST", loginPath, csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			showLogin(w, r, 400, "", h.localize(r, "err.invalid_login_body"))
			return
		}
		email := r.PostForm.Get("email")
		action := r.PostForm.Get("action")
		if action == "create_supplier" || action == "update_supplier" {
			h.browserSaveSupplier(w, r, cookies)
			return
		}
		if action == "create_budget" || action == "create_process" {
			h.browserCreatePanelRecord(w, r, cookies, action)
			return
		}

		if action == "resume" {
			authenticated, status := h.resumeSession(w, r, cookies)
			if authenticated {
				browserRedirect(w, r, myURL)
				return
			}
			if browserJSON(r) && status < 400 {
				browserRedirect(w, r, loginURL+"?resume=0")
				return
			}
			message := ""
			if status >= 500 {
				message = h.localize(r, "err.refresh_session")
			}
			showLogin(w, r, status, "", message)
			return
		}
		if action == "revoke" {
			request := r.Clone(r.Context())
			id := r.PostForm.Get("session_id")
			request.SetPathValue("id", id)
			current := false
			revoke := middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				identity, _ := middleware.AuthenticatedIdentity(r.Context())
				current = identity.Claims.SessionID == id
				h.RevokeSession(w, r)
			}))
			reply := browser.Call(request, revoke, nil, cookies.Read(r, "access"))
			if reply.Status == 401 {
				browserRedirect(w, r, loginURL)
				return
			}
			if reply.Status >= 400 {
				if browserJSON(r) {
					browserForward(w, reply)
					return
				}
				showLogin(w, r, reply.Status, "", h.localize(r, "err.revoke_session"))
				return
			}
			if current {
				cookies.Clear(w)
				browserRedirect(w, r, loginURL)
			} else if browserJSON(r) {
				httpx.WriteJSON(w, 200, map[string]any{"refreshSessions": true, "error": ""})
			} else {
				browserRedirect(w, r, myURL+"#account")
			}
			return
		}
		if action == "logout" {
			// A refresh cookie can expire before a still-valid access token.
			if access := cookies.Read(r, "access"); access != "" {
				claims, err := h.authenticator.Verify(r.Context(), access)
				if err != nil && !errors.Is(err, goauth.ErrInvalidToken) && !errors.Is(err, goauth.ErrExpiredToken) {
					h.logger.Error("failed to verify browser session before logout", "error", err)
					status, _ := httpx.ErrorStatus(r.Context(), http.StatusServiceUnavailable, "err.verify_bearer")
					showLogin(w, r, status, "", h.localize(r, "err.verify_bearer"))
					return
				}
				if err == nil {
					request := r.Clone(r.Context())
					request.SetPathValue("id", claims.SessionID)
					revoke := middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, http.HandlerFunc(h.RevokeSession))
					reply := browser.Call(request, revoke, nil, access)
					if reply.Status >= 400 {
						showLogin(w, r, reply.Status, "", h.localize(r, "err.revoke_session"))
						return
					}
				}
			}
			if token := cookies.Read(r, "refresh"); token != "" {
				if err := h.authenticator.Revoke(r.Context(), token); err != nil {
					h.writeError(
						w,
						r,
						http.StatusInternalServerError,
						err,
						"err.revoke_session",
						"failed to revoke browser session",
					)
					return
				}
			}
			cookies.Clear(w)
			browserRedirect(w, r, loginURL)
			return
		}
		var reply *browser.Reply
		switch action {
		case "login":
			reply = browser.Call(r, http.HandlerFunc(h.LoginOrCreateAccount), loginRequest{Email: email, Password: r.PostForm.Get("password")}, "")
		default:
			showLogin(w, r, 400, email, h.localize(r, "err.invalid_login_body"))
			return
		}
		if reply.Status >= 400 {
			var failure struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(reply.Body.Bytes(), &failure) != nil {
				failure.Error = h.localize(r, "err.finalize_login")
			}
			showLogin(w, r, reply.Status, email, failure.Error)
			return
		}
		var tokens browser.Tokens
		if err := json.Unmarshal(reply.Body.Bytes(), &tokens); err != nil || tokens.Access == "" || tokens.Refresh == "" {
			showLogin(w, r, 500, email, h.localize(r, "err.issue_tokens"))
			return
		}
		cookies.Save(w, tokens)
		w.Header().Set("Cache-Control", "no-store")
		browserRedirect(w, r, myURL)
	})))
	h.AddRoute(routes.MyPage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		h.browserMyPage(w, r, cookies, http.StatusOK)
	}))
	// Preserve existing account bookmarks while using one personal-area document.
	h.AddRoute(routes.MyAccountPage, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := myURL
		if cursor := r.URL.Query().Get("cursor"); cursor != "" {
			target += "?cursor=" + url.QueryEscape(cursor)
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target+"#account", http.StatusSeeOther)
	}))
}

// browserAuthenticated distinguishes invalid credentials from dependency failures.
func (h *Handler) browserAuthenticated(r *http.Request, cookies browser.Cookies) (bool, error) {
	token := cookies.Read(r, "access")
	if token == "" {
		return false, nil
	}
	claims, err := h.authenticator.Verify(r.Context(), token)
	if errors.Is(err, goauth.ErrInvalidToken) || errors.Is(err, goauth.ErrExpiredToken) {
		return false, nil
	}
	return err == nil && claims != nil, err
}

// resumeSession runs only after CSRF checking on POST. Recheck access after the
// browser acquires its origin-wide lock: another tab may already have renewed.
// Failures never clear cookies, because their response may arrive after a newer
// successful login/renewal. Only explicit sign out removes browser credentials.
func (h *Handler) resumeSession(w http.ResponseWriter, r *http.Request, cookies browser.Cookies) (bool, int) {
	if prefetch(r) {
		return false, http.StatusOK
	}
	authenticated, err := h.browserAuthenticated(r, cookies)
	if err != nil {
		h.logger.Error("failed to verify browser session before renewal", "error", err)
		status, _ := httpx.ErrorStatus(r.Context(), http.StatusServiceUnavailable, "err.verify_bearer")
		return false, status
	}
	if authenticated {
		return true, http.StatusOK
	}
	refresh := cookies.Read(r, "refresh")
	if refresh == "" {
		return false, http.StatusOK
	}
	reply := browser.Call(r, http.HandlerFunc(h.RefreshSession), refreshRequest{RefreshToken: refresh}, "")
	if reply.Status == http.StatusUnauthorized {
		return false, http.StatusOK
	}
	if reply.Status != http.StatusOK {
		return false, reply.Status
	}
	var tokens browser.Tokens
	if err := json.Unmarshal(reply.Body.Bytes(), &tokens); err != nil || tokens.Access == "" || tokens.Refresh == "" || tokens.AccessTTL <= 0 || tokens.RefreshTTL <= 0 {
		return false, http.StatusInternalServerError
	}
	cookies.Save(w, tokens)
	return true, http.StatusOK
}

// prefetch reports speculative requests whose response, including cookies, may
// be discarded. Renewing there would consume the rotating refresh token.
func prefetch(r *http.Request) bool {
	purpose := strings.ToLower(r.Header.Get("Sec-Purpose") + " " + r.Header.Get("Purpose"))
	return strings.Contains(purpose, "prefetch") || strings.Contains(purpose, "prerender")
}

// deviceNames extracts the browser and operating system shown as device icons.
func deviceNames(userAgent string) (browserName, osName string) {
	ua := useragent.New(userAgent)
	browserName, _ = ua.Browser()
	osName = ua.OSInfo().Name
	switch ua.Platform() {
	case "iPad", "iPhone", "iPod touch":
		osName = "iOS"
	}
	return browserName, osName
}
