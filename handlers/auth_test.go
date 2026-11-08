package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"sifeucr/auth"
	"sifeucr/database"
	"sifeucr/handlers"
	locales "sifeucr/i18n"
	"sifeucr/internal/testdb"
	"github.com/tavocg/go-email/mailer"
	"github.com/tavocg/go-i18n"
	"github.com/tavocg/go-secrets/otp"
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
