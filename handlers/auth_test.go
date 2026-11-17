package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tavocg/go-email/mailer"
	"github.com/tavocg/go-i18n"
	"github.com/tavocg/go-secrets/otp"
	"sifeucr/auth"
	"sifeucr/database"
	"sifeucr/handlers"
	locales "sifeucr/i18n"
	"sifeucr/internal/testdb"
)

type testMailer struct {
	messages chan *mailer.Message
	err      error
}

func (m testMailer) Send(ctx context.Context, message *mailer.Message) error {
	select {
	case m.messages <- message:
		return m.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type harness struct {
	handler http.Handler
	auth    *auth.Authenticator
	signer  *otp.Signer
	mail    testMailer
}

func newHarness(t *testing.T, db database.Database, deliveryErr error) harness {
	t.Helper()
	a, err := auth.NewAuthenticator(db, "test-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	l, err := i18n.NewLocalizer(locales.Locales())
	if err != nil {
		t.Fatal(err)
	}
	signer, err := otp.NewSigner("independent-test-otp-secret")
	if err != nil {
		t.Fatal(err)
	}
	mail := testMailer{make(chan *mailer.Message, 16), deliveryErr}
	h, err := handlers.NewHandler(handlers.Options{
		DB: db, Authenticator: a, Localizer: l, OTPSigner: signer,
		Mailer: mail, MailFrom: "test@example.com",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return harness{h, a, signer, mail}
}

func (h harness) request(method, path, body, token, peer string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = peer
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
	if want == http.StatusTooManyRequests {
		seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
		if err != nil || seconds <= 0 {
			t.Fatalf("invalid Retry-After: %q", w.Header().Get("Retry-After"))
		}
	}
}

func (h harness) code(t *testing.T) string {
	t.Helper()
	select {
	case message := <-h.mail.messages:
		code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(message.PlainBody)
		if code == "" {
			t.Fatal("mail did not contain a six-digit code")
		}
		return code
	default:
		t.Fatal("expected a verification email")
		return ""
	}
}

func TestLoginOTPIsHashedAndConsumedOnce(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		h := newHarness(t, db, nil)
		requireStatus(t, h.request("POST", "/auth/login", `{"email":"User+tag@Example.com"}`, "", "127.0.0.1:1"), 204)
		code := h.code(t)
		digest := h.signer.Sign("login", "user+tag@example.com", code)
		rollback := errors.New("inspect without consuming")
		err := db.WithTx(t.Context(), func(q database.Querier) error {
			saved, err := q.ConsumeAccountLoginRequest(t.Context(), "user+tag@example.com", digest, time.Now().Unix())
			if err != nil {
				return err
			}
			if saved.OTPHash == code || !h.signer.Verify("login", saved.Email, code, saved.OTPHash) {
				t.Error("database did not store the code as its context-bound digest")
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		body := `{"email":"user+tag@example.com","otp":"` + code + `"}`
		start, results := make(chan struct{}), make(chan *httptest.ResponseRecorder, 2)
		for range 2 {
			go func() { <-start; results <- h.request("POST", "/auth/verify", body, "", "127.0.0.1:1") }()
		}
		close(start)
		statuses := map[int]int{}
		for range 2 {
			w := <-results
			statuses[w.Code]++
			if w.Code == 200 {
				var tokens struct {
					AccessToken string `json:"access_token"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &tokens); err != nil {
					t.Error(err)
					continue
				}
				if _, err := h.auth.Verify(t.Context(), tokens.AccessToken); err != nil {
					t.Errorf("login returned an invalid session: %v", err)
				}
			}
		}
		if statuses[200] != 1 || statuses[401] != 1 {
			t.Fatalf("concurrent verification statuses = %v", statuses)
		}
		t.Run("native browser session", func(t *testing.T) {
			autoResumeForm := regexp.MustCompile(`<form\b[^>]*\bdata-auto-resume(?:[\s=>])`)
			var cookies []*http.Cookie
			call := func(method, path string, values url.Values, origin string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("User-Agent", "Browser integration test")
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				for _, cookie := range cookies {
					req.AddCookie(cookie)
				}
				out := httptest.NewRecorder()
				h.handler.ServeHTTP(out, req)
				return out
			}
			form := url.Values{"action": {"login"}, "email": {"browser@example.com"}}
			requireStatus(t, call("POST", "/login", form, "https://attacker.example"), 403)
			requireStatus(t, call("POST", "/login", form, "null"), 403)
			requireStatus(t, call("GET", "/home", nil, ""), 303)
			login := call("GET", "/login", nil, "")
			requireStatus(t, login, 200)
			if login.Header().Get("Referrer-Policy") != "strict-origin" {
				t.Fatal("referrer policy must preserve native form Origin on HTTP LAN hosts")
			}
			requireStatus(t, call("POST", "/login", form, "http://example.com"), 303)
			form.Set("action", "verify")
			form.Set("otp", "invalid")
			invalid := call("POST", "/login", form, "")
			requireStatus(t, invalid, 401)
			if !strings.Contains(invalid.Body.String(), `class="warn" role="alert">Invalid login verification code.`) || !strings.Contains(invalid.Body.String(), `id="login-otp"`) {
				t.Fatal("verification error was not visible beside the code input")
			}
			form.Set("otp", h.code(t))
			signed := call("POST", "/login", form, "")
			requireStatus(t, signed, 303)
			cookies = signed.Result().Cookies()
			if len(cookies) != 2 {
				t.Fatal("missing session cookies")
			}
			for _, cookie := range cookies {
				if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
					t.Fatal("unsafe cookie attributes")
				}
			}
			redirected := call("GET", "/login", nil, "")
			requireStatus(t, redirected, 303)
			if redirected.Header().Get("Location") != "/home" || redirected.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("authenticated login did not redirect safely")
			}
			home := call("GET", "/home", nil, "")
			requireStatus(t, home, 200)
			if !strings.Contains(home.Body.String(), `class="tag info"`) || !strings.Contains(home.Body.String(), "Current") {
				t.Fatal("home did not show current session")
			}
			requireStatus(t, call("GET", "/account/sessions", nil, ""), 401)
			// Speculative requests must not consume the rotating refresh token.
			expired := make([]*http.Cookie, 0, len(cookies))
			for _, cookie := range cookies {
				if !strings.HasSuffix(cookie.Name, "_access") {
					expired = append(expired, cookie)
				}
			}
			cookies = expired
			speculative := httptest.NewRequest("GET", "/login", nil)
			speculative.Header.Set("Sec-Purpose", "prefetch")
			for _, cookie := range cookies {
				speculative.AddCookie(cookie)
			}
			prefetched := httptest.NewRecorder()
			h.handler.ServeHTTP(prefetched, speculative)
			requireStatus(t, prefetched, 200)
			if len(prefetched.Result().Cookies()) != 0 {
				t.Fatal("prefetch consumed the refresh token")
			}
			// All GETs are read-only; the client submits the hidden renewal form.
			pending := call("GET", "/login", nil, "")
			requireStatus(t, pending, 200)
			if len(pending.Result().Cookies()) != 0 || !autoResumeForm.MatchString(pending.Body.String()) || strings.Contains(pending.Body.String(), "Continue session") {
				t.Fatal("GET did not offer safe automatic resumption")
			}
			requireStatus(t, call("POST", "/login", url.Values{"action": {"resume"}}, "https://attacker.example"), 403)
			staleCookies := append([]*http.Cookie(nil), cookies...)
			resumed := call("POST", "/login", url.Values{"action": {"resume"}}, "")
			requireStatus(t, resumed, 303)
			if resumed.Header().Get("Location") != "/home" {
				t.Fatal("expired access cookie did not resume the session")
			}
			cookies = resumed.Result().Cookies()
			if len(cookies) != 2 {
				t.Fatal("resumed session did not set new cookies")
			}
			// A queued tab reads the updated access cookie and must not rotate again.
			queued := call("POST", "/login", url.Values{"action": {"resume"}}, "")
			requireStatus(t, queued, 303)
			if len(queued.Result().Cookies()) != 0 {
				t.Fatal("queued renewal rotated an already valid session")
			}
			stale := httptest.NewRequest("POST", "/login", strings.NewReader("action=resume"))
			stale.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for _, cookie := range staleCookies {
				stale.AddCookie(cookie)
			}
			failed := httptest.NewRecorder()
			h.handler.ServeHTTP(failed, stale)
			requireStatus(t, failed, 200)
			if len(failed.Result().Cookies()) != 0 || autoResumeForm.MatchString(failed.Body.String()) {
				t.Fatal("stale renewal cleared cookies or restarted renewal")
			}
			requireStatus(t, call("GET", "/home", nil, ""), 200)
			var current *auth.Claims
			for _, cookie := range cookies {
				if strings.HasSuffix(cookie.Name, "_access") {
					current, err = h.auth.Verify(t.Context(), cookie.Value)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if current == nil {
				t.Fatal("missing current identity")
			}
			const otherDevice = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
			other, _, err := h.auth.Issue(t.Context(), &auth.Claims{Sub: current.Sub, UserAgent: otherDevice})
			if err != nil {
				t.Fatal(err)
			}
			otherClaims, err := h.auth.Verify(t.Context(), other.Value)
			if err != nil {
				t.Fatal(err)
			}
			confirmation := call("GET", "/home", nil, "")
			requireStatus(t, confirmation, 200)
			if !strings.Contains(confirmation.Body.String(), `<dialog id="revoke-`+otherClaims.SessionID+`"`) {
				t.Fatal("missing session revocation modal")
			}
			modal := regexp.MustCompile(`(?s)<dialog id="revoke-` + otherClaims.SessionID + `".*?</dialog>`).FindString(confirmation.Body.String())
			if !strings.Contains(modal, "Log out of device:") || !strings.Contains(modal, "Chrome (Windows)") || strings.Contains(modal, otherDevice) {
				t.Fatal("confirmation did not identify the selected device")
			}
			if _, err := h.auth.Verify(t.Context(), other.Value); err != nil {
				t.Fatal("GET revoked a session")
			}
			revokeForm := url.Values{"action": {"revoke"}, "session_id": {otherClaims.SessionID}}
			requireStatus(t, call("POST", "/login", revokeForm, "https://attacker.example"), 403)
			requireStatus(t, call("POST", "/login", revokeForm, ""), 303)
			if _, err := h.auth.Verify(t.Context(), other.Value); err == nil {
				t.Fatal("other session still authenticates")
			}
			requireStatus(t, call("GET", "/home", nil, ""), 200)
			requireStatus(t, call("POST", "/login", url.Values{"action": {"logout"}}, ""), 303)
			// Old cookies must stop authenticating after server-side revocation.
			requireStatus(t, call("GET", "/home", nil, ""), 303)
			requireStatus(t, call("GET", "/login", nil, ""), 200)
			access, refresh, err := h.auth.Issue(t.Context(), &auth.Claims{Sub: current.Sub})
			if err != nil {
				t.Fatal(err)
			}
			claims, err := h.auth.Verify(t.Context(), access.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, cookie := range cookies {
				if strings.HasSuffix(cookie.Name, "_access") {
					cookie.Value = access.Value
				} else {
					cookie.Value = refresh.Value
				}
			}
			own := call("POST", "/login", url.Values{"action": {"revoke"}, "session_id": {claims.SessionID}}, "")
			requireStatus(t, own, 303)
			if own.Header().Get("Location") != "/login" {
				t.Fatal("revoking current session did not redirect to login")
			}
			for _, cookie := range own.Result().Cookies() {
				if cookie.MaxAge != -1 {
					t.Fatal("current cookies were not cleared")
				}
			}
			requireStatus(t, call("GET", "/home", nil, ""), 303)
			requireStatus(t, call("GET", "/login", nil, ""), 200)
			// Dependency failures must remain errors, preserving existing credentials.
			at, rt, err := h.auth.Issue(t.Context(), &auth.Claims{Sub: current.Sub})
			if err != nil {
				t.Fatal(err)
			}
			for _, cookie := range cookies {
				if strings.HasSuffix(cookie.Name, "_access") {
					cookie.Value = at.Value
				} else {
					cookie.Value = rt.Value
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			unavailable := call("GET", "/login", nil, "")
			requireStatus(t, unavailable, 503)
			if len(unavailable.Result().Cookies()) != 0 || autoResumeForm.MatchString(unavailable.Body.String()) {
				t.Fatal("backend failure mutated credentials or initiated renewal")
			}

		})

	})
}

func TestOTPBudgetsFollowIdentityAcrossIPs(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		h := newHarness(t, db, nil)
		requireStatus(t, h.request("POST", "/auth/login", `{"email":"owner@example.com"}`, "", "127.0.0.1:1"), 204)
		code := h.code(t)
		requireStatus(t, h.request("POST", "/auth/login", `{"email":"OWNER@example.com"}`, "", "192.0.2.1:1"), 429)
		wrong := "000000"
		if wrong == code {
			wrong = "111111"
		}
		for i := range 5 {
			requireStatus(t, h.request("POST", "/auth/verify", `{"email":"owner@example.com","otp":"`+wrong+`"}`, "", "192.0.2."+strconv.Itoa(i+1)+":1"), 401)
		}
		requireStatus(t, h.request("POST", "/auth/verify", `{"email":"owner@example.com","otp":"`+code+`"}`, "", "192.0.2.99:1"), 429)
		// Independent identities behind the same proxy retain their allowances.
		for i := range 12 {
			body := `{"email":"proxy` + strconv.Itoa(i) + `@example.com"}`
			requireStatus(t, h.request("POST", "/auth/login", body, "", "127.0.0.1:1"), 204)
		}
	})
}

func TestLoginRejectsExpiredAndCrossContextDigests(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		h := newHarness(t, db, nil)
		for _, name := range []string{"expired", "wrong-purpose", "wrong-identity"} {
			email, purpose, bound := name+"@example.com", "login", name+"@example.com"
			expiry := time.Now().Add(time.Hour).Unix()
			switch name {
			case "expired":
				expiry = time.Now().Unix() - 1
			case "wrong-purpose":
				purpose = "email_change"
			case "wrong-identity":
				bound = "someone-else@example.com"
			}
			if err := db.Querier().UpsertAccountLoginRequest(t.Context(), email, h.signer.Sign(purpose, bound, "123456"), expiry); err != nil {
				t.Fatal(err)
			}
			requireStatus(t, h.request("POST", "/auth/verify", `{"email":"`+email+`","otp":"123456"}`, "", "127.0.0.1:1"), 401)
		}
	})
}

func TestDeliveryFailurePreservesChallengeAndCooldown(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		h := newHarness(t, db, errors.New("provider may already have accepted the message"))
		requireStatus(t, h.request("POST", "/auth/login", `{"email":"owner@example.com"}`, "", "127.0.0.1:1"), 502)
		code := h.code(t)
		requireStatus(t, h.request("POST", "/auth/login", `{"email":"owner@example.com"}`, "", "127.0.0.1:1"), 429)
		requireStatus(t, h.request("POST", "/auth/verify", `{"email":"owner@example.com","otp":"`+code+`"}`, "", "127.0.0.1:1"), 200)
	})
}

func TestEmailConflictRollsBackOTPAndPreservesProfile(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		h := newHarness(t, db, nil)
		q := db.Querier()
		sub, err := q.EnsureAccountByEmail(t.Context(), "owner@example.com", time.Now().Unix())
		if err != nil {
			t.Fatal(err)
		}
		target, err := q.EnsureAccountByEmail(t.Context(), "taken@example.com", time.Now().Unix())
		if err != nil {
			t.Fatal(err)
		}
		if err := q.AssignRoleToAccount(t.Context(), sub, database.RoleDefault); err != nil {
			t.Fatal(err)
		}
		at, _, err := h.auth.Issue(t.Context(), &auth.Claims{Sub: strconv.FormatInt(sub, 10)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.SelectAccountBySub(t.Context(), sub); err != nil { // Warm the profile cache.
			t.Fatal(err)
		}
		requireStatus(t, h.request("PATCH", "/account/email/change", `{"new_email":"taken@example.com"}`, at.Value, "127.0.0.1:1"), 204)
		body := `{"otp":"` + h.code(t) + `"}`
		requireStatus(t, h.request("PATCH", "/account/email/change/confirm", body, at.Value, "127.0.0.1:1"), 409)
		profile, err := q.SelectAccountBySub(t.Context(), sub)
		if err != nil || profile.Email != "owner@example.com" {
			t.Fatalf("conflict changed profile: %v, %v", profile, err)
		}
		if err := q.UpdateAccountEmail(t.Context(), target, "released@example.com"); err != nil {
			t.Fatal(err)
		}
		requireStatus(t, h.request("PATCH", "/account/email/change/confirm", body, at.Value, "127.0.0.1:1"), 204)
		profile, err = q.SelectAccountBySub(t.Context(), sub)
		if err != nil || profile.Email != "taken@example.com" {
			t.Fatalf("successful change not visible: %v, %v", profile, err)
		}
		requireStatus(t, h.request("PATCH", "/account/email/change/confirm", body, at.Value, "127.0.0.1:1"), 401)
	})
}
