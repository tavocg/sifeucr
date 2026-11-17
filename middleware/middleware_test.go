package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	goauth "github.com/tavocg/go-auth"
	appauth "sifeucr/auth"
	"sifeucr/middleware"
)

type fakeAuthenticator struct {
	claims *appauth.Claims
	err    error
}

func (f *fakeAuthenticator) Issue(context.Context, *appauth.Claims) (*goauth.Token, *goauth.Token, error) {
	return nil, nil, errors.New("unused")
}

func (f *fakeAuthenticator) Refresh(context.Context, string) (*goauth.Token, *goauth.Token, error) {
	return nil, nil, errors.New("unused")
}

func (f *fakeAuthenticator) Verify(context.Context, string) (*appauth.Claims, error) {
	return f.claims, f.err
}

func (f *fakeAuthenticator) Revoke(context.Context, string) error {
	return errors.New("unused")
}

func (f *fakeAuthenticator) RevokeAll(context.Context, *appauth.Claims) error {
	return errors.New("unused")
}

type fakeChecker struct {
	allowed    bool
	err        error
	calls      int
	sub        int64
	permission string
}

func (c *fakeChecker) AccountHasPermission(_ context.Context, sub int64, permission string) (bool, error) {
	c.calls++
	c.sub, c.permission = sub, permission
	return c.allowed, c.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testLocalize(_ *http.Request, key string, _ ...any) string {
	return "localized:" + key
}

func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, status, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error != message {
		t.Errorf("error body = %q, want %q", body.Error, message)
	}
}

func TestAuthenticateBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := middleware.AuthenticatedIdentity(r.Context())
		if !ok {
			t.Error("next handler did not receive an identity")
			return
		}
		if identity.Sub != 42 {
			t.Errorf("identity subject = %d, want 42", identity.Sub)
		}
		if identity.Claims == nil || identity.Claims.Subject() != "42" {
			t.Errorf("identity claims = %#v, want subject 42", identity.Claims)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name       string
		header     string
		auth       *fakeAuthenticator
		wantStatus int
		wantKey    string
	}{
		{
			name:       "missing header",
			auth:       &fakeAuthenticator{},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.missing_bearer",
		},
		{
			name:       "malformed scheme",
			header:     "Basic abc",
			auth:       &fakeAuthenticator{},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.missing_bearer",
		},
		{
			name:       "malformed token",
			header:     "Bearer a b",
			auth:       &fakeAuthenticator{},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.missing_bearer",
		},
		{
			name:       "invalid token",
			header:     "Bearer invalid",
			auth:       &fakeAuthenticator{err: goauth.ErrInvalidToken},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.verify_bearer",
		},
		{
			name:       "expired token",
			header:     "Bearer expired",
			auth:       &fakeAuthenticator{err: goauth.ErrExpiredToken},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.verify_bearer",
		},
		{
			name:       "wrapped invalid token",
			header:     "Bearer invalid",
			auth:       &fakeAuthenticator{err: errors.Join(errors.New("wrapped"), goauth.ErrInvalidToken)},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.verify_bearer",
		},
		{
			name:       "verification failure",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{err: errors.New("database unavailable")},
			wantStatus: http.StatusInternalServerError,
			wantKey:    "err.verify_bearer",
		},
		{
			name:       "nil claims",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.missing_identity",
		},
		{
			name:       "missing subject",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{claims: &appauth.Claims{}},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.missing_identity",
		},
		{
			name:       "non-numeric subject",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{claims: &appauth.Claims{Sub: "abc"}},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.invalid_subject",
		},
		{
			name:       "non-positive subject",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{claims: &appauth.Claims{Sub: "0"}},
			wantStatus: http.StatusUnauthorized,
			wantKey:    "err.invalid_subject",
		},
		{
			name:       "valid token",
			header:     "Bearer token",
			auth:       &fakeAuthenticator{claims: &appauth.Claims{Sub: "42"}},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			middleware.AuthenticateBearer(testLogger(), testLocalize, tt.auth, next).ServeHTTP(rec, req)

			if tt.wantKey == "" {
				if rec.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
				}
				return
			}
			assertJSONError(t, rec, tt.wantStatus, "localized:"+tt.wantKey)
		})
	}
}

func TestRequirePermission(t *testing.T) {
	nextCalls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	})

	t.Run("missing identity", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		checker := &fakeChecker{allowed: true}

		middleware.RequirePermission(testLogger(), testLocalize, checker, "perm.test", next).ServeHTTP(rec, req)

		assertJSONError(t, rec, http.StatusUnauthorized, "localized:err.missing_identity")
		if checker.calls != 0 || nextCalls != 0 {
			t.Fatal("missing identity reached the permission checker or protected handler")
		}
	})

	tests := []struct {
		name       string
		checker    fakeChecker
		wantStatus int
		wantKey    string
	}{
		{
			name:       "allowed",
			checker:    fakeChecker{allowed: true},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "denied",
			checker:    fakeChecker{allowed: false},
			wantStatus: http.StatusForbidden,
			wantKey:    "err.permission_denied",
		},
		{
			name:       "checker failure",
			checker:    fakeChecker{err: errors.New("database unavailable")},
			wantStatus: http.StatusInternalServerError,
			wantKey:    "err.check_role_permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalls = 0
			authenticated := middleware.AuthenticateBearer(
				testLogger(),
				testLocalize,
				&fakeAuthenticator{claims: &appauth.Claims{Sub: "7"}},
				middleware.RequirePermission(testLogger(), testLocalize, &tt.checker, "perm.test", next),
			)

			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			authenticated.ServeHTTP(rec, req)
			if tt.checker.calls != 1 || tt.checker.sub != 7 || tt.checker.permission != "perm.test" {
				t.Fatalf("permission lookup = %d calls for subject %d, permission %q", tt.checker.calls, tt.checker.sub, tt.checker.permission)
			}
			wantCalls := 0
			if tt.wantStatus == http.StatusNoContent {
				wantCalls = 1
			}
			if nextCalls != wantCalls {
				t.Errorf("protected handler calls = %d, want %d", nextCalls, wantCalls)
			}

			if tt.wantKey == "" {
				if rec.Code != tt.wantStatus {
					t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
				}
				return
			}
			assertJSONError(t, rec, tt.wantStatus, "localized:"+tt.wantKey)
		})
	}
}

func TestRequestLoggerPreservesResponseController(t *testing.T) {
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("flush through request logger: %v", err)
		}
	})

	middleware.RequestLogger(testLogger(), next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Fatal("ResponseController did not reach the underlying writer")
	}
}
