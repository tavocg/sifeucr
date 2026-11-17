// Package handlers registers the application's HTTP handlers.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tavocg/go-auth"
	email "github.com/tavocg/go-email"
	"github.com/tavocg/go-i18n"
	appauth "sifeucr/auth"
	"sifeucr/database"
	"sifeucr/internal/logging"
	"sifeucr/middleware"
	"sifeucr/routes"
)

const requestTimeout = 20 * time.Second

type CredentialsVerifier func(context.Context, email.CredentialsConfig, string, string) error

type Handler struct {
	verifyCredentials CredentialsVerifier
	dev               bool

	logger        logging.Logger
	db            database.Database
	authenticator auth.Authenticator[*appauth.Claims]
	localizer     *i18n.Localizer
	links         routes.Links
	corsOrigins   []string
	csrfOrigins   []string
	namedRoutes   map[routes.ID]bool
	routePolicies map[string]routePolicy
	methods       map[string]bool
	credentials   email.CredentialsConfig

	mux   *http.ServeMux
	entry http.Handler
}

type Options struct {
	VerifyCredentials CredentialsVerifier
	Logger            logging.Logger
	Dev               bool
	DB                database.Database
	Authenticator     auth.Authenticator[*appauth.Claims]
	Localizer         *i18n.Localizer
	RootPrefix        string
	CORSOrigins       []string
	// CSRFOrigins lists origins allowed to post browser forms. The
	// cross-origin protection compares Origin with Host by default; tunnels and
	// proxies that rewrite Host must declare the public origin here instead.
	CSRFOrigins []string
	Credentials email.CredentialsConfig
}

func NewHandler(opts Options) (*Handler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	if opts.DB == nil {
		return nil, errors.New("handler database is required")
	}
	if opts.Localizer == nil {
		return nil, errors.New("handler localizer is required")
	}

	if opts.Authenticator == nil {
		return nil, errors.New("handler authenticator is required")
	}

	if !opts.Dev && opts.Credentials.Address == "" {
		return nil, errors.New("institutional SMTP address is required outside development")
	}

	links, err := routes.NewLinks(opts.RootPrefix)
	if err != nil {
		return nil, err
	}

	if err := validateOrigins(opts.CORSOrigins); err != nil {
		return nil, err
	}
	if err := validateOrigins(opts.CSRFOrigins); err != nil {
		return nil, err
	}

	verifier := opts.VerifyCredentials
	if verifier == nil {
		verifier = email.VerifyCredentials
	}
	next := &Handler{
		verifyCredentials: verifier,
		corsOrigins:       append([]string(nil), opts.CORSOrigins...),
		csrfOrigins:       append([]string(nil), opts.CSRFOrigins...),
		namedRoutes:       make(map[routes.ID]bool),
		routePolicies:     make(map[string]routePolicy),
		methods:           make(map[string]bool),
		dev:               opts.Dev,
		logger:            logger,
		db:                opts.DB,
		authenticator:     opts.Authenticator,
		localizer:         opts.Localizer,
		links:             links,
		credentials:       opts.Credentials,
		mux:               http.NewServeMux(),
	}

	next.registerRoutes()
	if err := next.validateRoutes(); err != nil {
		return nil, err
	}
	next.entry = middleware.RequestLogger(next.logger, http.HandlerFunc(next.route))

	return next, nil
}

// Add registers a handler function relative to the configured root prefix.
func (h *Handler) Add(method, path string, fn http.HandlerFunc, options ...RouteOptions) {
	h.AddHandler(method, path, fn, options...)
}

// AddHandler registers a handler with a request deadline, including its middleware.
func (h *Handler) AddHandler(method, path string, handler http.Handler, options ...RouteOptions) {
	if len(options) > 1 {
		panic("only one RouteOptions value is supported")
	}
	origins := h.corsOrigins
	if len(options) == 1 && options[0].CORSOrigins != nil {
		origins = options[0].CORSOrigins
	}
	if err := validateOrigins(origins); err != nil {
		panic(err)
	}
	origins = append([]string(nil), origins...)
	pattern := h.links.Mount(path)
	if method != "" {
		pattern = method + " " + pattern
	}

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.corsHeaders(w, r, origins)
		handler.ServeHTTP(w, r)
	})
	h.mux.Handle(pattern, middleware.WithTimeout(requestTimeout, wrapped))
	h.routePolicies[pattern] = routePolicy{method: method, origins: origins}
	h.methods[method] = true
	if method == http.MethodGet {
		h.methods[http.MethodHead] = true
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.entry == nil {
		panic("handler not initialized")
	}

	h.entry.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.registerAuthRoutes()
	h.registerAccountRoutes()
	h.registerRootRoutes()
	h.registerInformationRoutes()
	h.registerStaticRoutes()
	h.registerBrowserRoutes()
}
