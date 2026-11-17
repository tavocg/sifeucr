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

	email "github.com/tavocg/go-email"
	"github.com/tavocg/go-i18n"
	"sifeucr/auth"
	"sifeucr/database/sqlite"
	locales "sifeucr/i18n"
	"sifeucr/routes"
	"sifeucr/templates"
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

	return Options{
		Credentials:   email.CredentialsConfig{Address: "smtp.example.com:465"},
		Logger:        testLogger(),
		DB:            db,
		Authenticator: authenticator,
		Localizer:     localizer,
		RootPrefix:    rootPrefix,
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
		{name: "browser login", method: "GET", path: "/api/login", wantStatus: 200, wantBody: `action="/api/login"`},
		{name: "pinned Datastar", method: "GET", path: "/api/s/" + templates.DatastarScriptPath, wantStatus: 200},
		{name: "protected browser home", method: "GET", path: "/api/dashboard", wantStatus: 303, wantLocation: "/api/login"},
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
			if tt.name == "browser login" {
				if strings.Contains(rec.Body.String(), "<script") {
					t.Error("ordinary login must not load scripts")
				}
				if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "'"+templates.NativeBusyHash+"'") {
					t.Error("login CSP did not authorize the native busy handler")
				}
			}
			if tt.wantJSON {
				if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				if got := rec.Header().Get("Cache-Control"); got != "no-store" {
					t.Errorf("Cache-Control = %q, want no-store", got)
				}
			}
			if tt.wantAllow && rec.Header().Get("Allow") != "GET, HEAD, OPTIONS" {
				t.Errorf("Allow = %q, want GET, HEAD, OPTIONS", rec.Header().Get("Allow"))
			}
			if tt.wantLocation != "" && rec.Header().Get("Location") != tt.wantLocation {
				t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), tt.wantLocation)
			}
		})
	}
	t.Run("named routes and CORS", func(t *testing.T) {
		opts := testHandlerOptions(t, "/api")
		opts.CORSOrigins = []string{"https://default.example"}
		h, err := NewHandler(opts)
		if err != nil {
			t.Fatal(err)
		}
		link, err := h.URL(routes.APIDocs)
		if err != nil || link != "/api/s/api/" {
			t.Fatalf("docs link = %q, %v", link, err)
		}
		if _, err := h.URL(routes.RevokeSession); err == nil {
			t.Fatal("missing parameter accepted")
		}
		delete(h.namedRoutes, routes.Login)
		if err := h.validateRoutes(); err == nil {
			t.Fatal("missing route accepted")
		}
		h.namedRoutes[routes.Login] = true
		endpoint := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }
		h.Add("GET", "/cors", endpoint)
		h.Add("POST", "/cors", endpoint, RouteOptions{CORSOrigins: []string{"https://override.example"}})
		h.Add("GET", "/disabled", endpoint, RouteOptions{CORSOrigins: []string{}})
		for _, tc := range []struct {
			method, path, origin, requested, headers string
			status                                   int
			cors                                     bool
		}{
			{"GET", "/api/cors", "https://default.example", "", "", 204, true},
			{"POST", "/api/cors", "https://default.example", "", "", 204, false},
			{"POST", "/api/cors", "https://override.example", "", "", 204, true},
			{"GET", "/api/disabled", "https://default.example", "", "", 204, false},
			{"OPTIONS", "/api/cors", "", "", "", 204, false},
			{"OPTIONS", "/api/cors", "https://default.example", "GET", "Authorization, Content-Type", 204, true},
			{"OPTIONS", "/api/cors", "https://default.example", "POST", "", 403, false},
			{"OPTIONS", "/api/cors", "https://override.example", "POST", "", 204, true},
			{"OPTIONS", "/api/cors", "https://override.example", "DELETE", "", 403, false},
			{"OPTIONS", "/api/cors", "https://default.example", "GET", "X-Not-Allowed", 403, false},
			{"OPTIONS", "/api/disabled", "https://default.example", "GET", "", 403, false},
			{"OPTIONS", "/api/account", "https://default.example", "GET", "Authorization", 204, true},
			{"OPTIONS", "/cors", "https://default.example", "GET", "", 404, false},
			{"OPTIONS", "/api/missing", "https://default.example", "GET", "", 404, false},
		} {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", tc.requested)
			req.Header.Set("Access-Control-Request-Headers", tc.headers)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || (rec.Header().Get("Access-Control-Allow-Origin") != "") != tc.cors {
				t.Fatalf("%+v: status %d, headers %v", tc, rec.Code, rec.Header())
			}
		}
	})

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
	// Independent handlers must render and serve their own links, including on 404.
	handlers := []*Handler{newRoutingHandler(t, ""), newRoutingHandler(t, "/api"), newRoutingHandler(t, "/other")}
	for _, h := range handlers {
		rootURL, _ := h.URL(routes.Root)
		docsURL, _ := h.URL(routes.APIDocs)
		cssURL, err := templates.SemanticStylesheetURL(h.links)
		if err != nil {
			t.Fatal(err)
		}
		for _, page := range []struct {
			path   string
			status int
		}{
			{rootURL, 200}, {h.links.Mount("/missing/deep/path"), 404},
		} {
			req := httptest.NewRequest("GET", page.path, nil)
			req.Header.Set("Accept", "text/html")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != page.status || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("page %s: %d %s", page.path, rec.Code, rec.Body.String())
			}
			if strings.Count(rec.Body.String(), `<style id="semantic-style">`) != 1 {
				t.Fatalf("page %s must include the stylesheet exactly once", page.path)
			}
			if !strings.Contains(rec.Body.String(), `href="`+docsURL+`"`) {
				t.Fatalf("page %s missing link %q", page.path, docsURL)
			}
		}
		for _, link := range []string{docsURL, cssURL} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", link, nil))
			if rec.Code != 200 {
				t.Fatalf("linked resource %s: %d", link, rec.Code)
			}
		}
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

func TestLinksRootPrefix(t *testing.T) {
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
			links, err := routes.NewLinks(tt.prefix)
			got, _ := links.URL(routes.Root)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewLinks(%q) = %q, want error", tt.prefix, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewLinks(%q): %v", tt.prefix, err)
			}
			if tt.want == "" {
				tt.want = "/"
			}
			if got != tt.want {
				t.Errorf("NewLinks(%q) = %q, want %q", tt.prefix, got, tt.want)
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

	t.Run("includes automatic OPTIONS on router 405", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api", nil)
		w := &routingErrorWriter{ResponseWriter: rec, handler: h, request: req}
		rec.Header().Set("Allow", "GET")

		w.WriteHeader(http.StatusMethodNotAllowed)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != "GET, OPTIONS" {
			t.Errorf("Allow = %q, want GET, OPTIONS", got)
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
