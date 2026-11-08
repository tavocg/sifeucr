package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sifeucr/auth"
	"sifeucr/database/sqlite"
	locales "sifeucr/i18n"
	"sifeucr/mailer/noop"
	"github.com/tavocg/go-i18n"
	"github.com/tavocg/go-secrets/otp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testHandlerOptions(t *testing.T, rootPrefix string) Options {
	t.Helper()

	db, err := sqlite.NewSQLite(t.Context(), filepath.Join(t.TempDir(), "sifeucr.sqlite"), sqlite.WithForeignKeys())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	authenticator, err := auth.NewAuthenticator(db, "test-jwt-secret", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	localizer, err := i18n.NewLocalizer(locales.Locales())
	if err != nil {
		t.Fatal(err)
	}
	signer, err := otp.NewSigner("independent-test-otp-secret")
	if err != nil {
		t.Fatal(err)
	}

	return Options{
		Logger:        testLogger(),
		DB:            db,
		Authenticator: authenticator,
		Localizer:     localizer,
		RootPrefix:    rootPrefix,
		Mailer:        noop.Mailer{},
		MailFrom:      "test@example.com",
		OTPSigner:     signer,
	}
}

func newRoutingHandler(t *testing.T, rootPrefix string) *Handler {
	t.Helper()

	h, err := NewHandler(testHandlerOptions(t, rootPrefix))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestRoutesUseRootPrefix(t *testing.T) {
	h := newRoutingHandler(t, "/api")

	tests := []struct {
		name         string
		method       string
		path         string
		wantStatus   int
		wantBody     string
		wantJSON     bool
		wantAllow    bool
		wantLocation string
	}{
		{
			name:       "root JSON",
			method:     http.MethodGet,
			path:       "/api",
			wantStatus: http.StatusOK,
			wantBody:   `"Hello, world!"`,
			wantJSON:   true,
		},
		{
			name:       "unknown route",
			method:     http.MethodGet,
			path:       "/api/missing",
			wantStatus: http.StatusNotFound,
			wantBody:   "Route not found.",
			wantJSON:   true,
		},
		{
			name:       "wrong method",
			method:     http.MethodPost,
			path:       "/api",
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "Method not allowed for this route.",
			wantJSON:   true,
			wantAllow:  true,
		},
		{
			name:       "static file",
			method:     http.MethodGet,
			path:       "/api/s/hello.txt",
			wantStatus: http.StatusOK,
			wantBody:   "hello, world!",
		},
		{
			name:         "static directory redirect",
			method:       http.MethodGet,
			path:         "/api/s",
			wantStatus:   http.StatusTemporaryRedirect,
			wantLocation: "/api/s/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want containing %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantJSON {
				if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				if got := rec.Header().Get("Cache-Control"); got != "no-store" {
					t.Errorf("Cache-Control = %q, want no-store", got)
				}
			}
			if tt.wantAllow && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q, want GET, HEAD", rec.Header().Get("Allow"))
			}
			if tt.wantLocation != "" && rec.Header().Get("Location") != tt.wantLocation {
				t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), tt.wantLocation)
			}
		})
	}
}

func TestRootWithoutPrefix(t *testing.T) {
	h := newRoutingHandler(t, "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Hello, world!"`) {
		t.Fatalf("root response = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", rec.Code)
	}
}

func TestRouteParametersAndResponseController(t *testing.T) {
	h := newRoutingHandler(t, "/api")
	h.Add(http.MethodGet, "/widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, r.PathValue("id")); err != nil {
			t.Fatal(err)
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/widgets/42", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "42" || !rec.Flushed {
		t.Fatalf("wrapped response = %d %q, flushed %v", rec.Code, rec.Body.String(), rec.Flushed)
	}
}

func TestNormalizeRootPrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		want    string
		wantErr bool
	}{
		{name: "empty", prefix: "", want: ""},
		{name: "root", prefix: "/", want: ""},
		{name: "adds leading slash", prefix: "api", want: "/api"},
		{name: "removes trailing slash", prefix: "/api/", want: "/api"},
		{name: "trims whitespace", prefix: "  /api  ", want: "/api"},
		{name: "nested path", prefix: "/v1/api", want: "/v1/api"},
		{name: "root made of slashes", prefix: "//", want: ""},
		{name: "double leading slash", prefix: "//api", wantErr: true},
		{name: "repeated slash", prefix: "/api//v1", wantErr: true},
		{name: "dot segment", prefix: "/api/../v1", wantErr: true},
		{name: "single dot segment", prefix: "/api/./v1", wantErr: true},
		{name: "percent encoding", prefix: "/api/%31", wantErr: true},
		{name: "query string", prefix: "/api?x=1", wantErr: true},
		{name: "fragment", prefix: "/api#top", wantErr: true},
		{name: "backslash", prefix: `/api\v1`, wantErr: true},
		{name: "space", prefix: "/api v1", wantErr: true},
		{name: "tab", prefix: "/api\tv1", wantErr: true},
		{name: "wildcard", prefix: "/api/{v1}", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRootPrefix(tt.prefix)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeRootPrefix(%q) = %q, want error", tt.prefix, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeRootPrefix(%q): %v", tt.prefix, err)
			}
			if got != tt.want {
				t.Errorf("normalizeRootPrefix(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestNewHandlerRejectsInvalidRootPrefix(t *testing.T) {
	_, err := NewHandler(testHandlerOptions(t, "/api//v1"))
	if err == nil {
		t.Fatal("NewHandler accepted an invalid root prefix")
	}
}

func TestRoutingErrorWriter(t *testing.T) {
	h := &Handler{logger: testLogger()}

	t.Run("replaces router 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		w := &routingErrorWriter{ResponseWriter: rec, handler: h, request: req}

		w.WriteHeader(http.StatusNotFound)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "err.route_not_found") {
			t.Errorf("body = %q, want localized route error", rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
	})

	t.Run("preserves Allow on router 405", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api", nil)
		w := &routingErrorWriter{ResponseWriter: rec, handler: h, request: req}
		rec.Header().Set("Allow", "GET")

		w.WriteHeader(http.StatusMethodNotAllowed)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != "GET" {
			t.Errorf("Allow = %q, want GET", got)
		}
	})

	t.Run("preserves matched endpoint response", func(t *testing.T) {
		for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusTeapot} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/matched", nil)
			req.Pattern = "/matched"
			w := &routingErrorWriter{ResponseWriter: rec, handler: h, request: req}
			w.WriteHeader(status)
			if _, err := w.Write([]byte("endpoint error")); err != nil {
				t.Fatal(err)
			}
			if rec.Code != status || rec.Body.String() != "endpoint error" {
				t.Errorf("response = %d %q, want %d endpoint error", rec.Code, rec.Body.String(), status)
			}
		}
	})

	t.Run("ignores body after replacement", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		w := &routingErrorWriter{ResponseWriter: rec, handler: h, request: req}

		w.WriteHeader(http.StatusNotFound)
		n, err := w.Write([]byte("ignored"))
		if err != nil {
			t.Fatal(err)
		}
		if n != len("ignored") {
			t.Errorf("Write returned %d, want %d", n, len("ignored"))
		}
		if strings.Contains(rec.Body.String(), "ignored") {
			t.Errorf("body = %q, want router error only", rec.Body.String())
		}
	})
}
