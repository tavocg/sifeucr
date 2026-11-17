package cmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	email "github.com/tavocg/go-email"
	"github.com/tavocg/go-i18n"
	"sifeucr/auth"
	"sifeucr/cache/memory"
	"sifeucr/database"
	"sifeucr/database/cached"
	"sifeucr/database/provider"
	"sifeucr/handlers"
	locales "sifeucr/i18n"
	"sifeucr/internal/health"
)

type serverConfig struct {
	AuthSMTPAddress  string
	AuthSMTPStartTLS bool
	DrainDelay       time.Duration
	ShutdownTimeout  time.Duration
	DatabaseDSN      string
	AutoMigrate      bool
	MaxConnections   int
	CacheEnabled     bool
	Dev              bool
	LogLevel         string
	LogFormat        string
	AuthSecret       string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	RootPrefix       string
	CORSOrigins      []string
	CSRFOrigins      []string
	Host             string
	Port             int
}

func runServer(cfg serverConfig) error {
	if cfg.Dev {
		cfg.DrainDelay = 0
	}
	if cfg.DrainDelay < 0 || cfg.ShutdownTimeout <= 0 || cfg.MaxConnections < 1 {
		return errors.New("drain delay must be nonnegative; shutdown timeout and database connections must be positive")
	}

	logger, err := newLogger(cfg.Dev, cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return err
	}
	if cfg.Dev {
		logger.Warn("development mode allows all origins, bypasses browser CSRF checks and skips institutional credential verification")
	}

	if len(cfg.AuthSecret) < 32 && !cfg.Dev {
		return errors.New("auth.secret must contain at least 32 bytes outside development mode")
	}
	if !cfg.Dev {
		host, port, err := net.SplitHostPort(cfg.AuthSMTPAddress)
		if err != nil || host == "" || port == "" {
			return errors.New("auth.smtp.address must be host:port outside development mode")
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var db database.Database
	db, err = provider.Open(ctx, cfg.DatabaseDSN, database.OpenOptions{SkipMigrations: !cfg.AutoMigrate, MaxConnections: cfg.MaxConnections})
	if err != nil {
		return err
	}
	if cfg.CacheEnabled {
		cacheStore := memory.New()
		defer cacheStore.Close()
		db = cached.New(db, cacheStore)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database close error", "error", err)
		}
	}()

	localizer, err := i18n.NewLocalizer(locales.Locales())
	if err != nil {
		return err
	}

	authenticator, err := auth.NewAuthenticator(db, cfg.AuthSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		return err
	}

	h, err := handlers.NewHandler(handlers.Options{
		Logger:        logger,
		Dev:           cfg.Dev,
		DB:            db,
		Authenticator: authenticator,
		Localizer:     localizer,
		RootPrefix:    cfg.RootPrefix,
		CORSOrigins:   cfg.CORSOrigins,
		CSRFOrigins:   cfg.CSRFOrigins,
		Credentials:   email.CredentialsConfig{Address: cfg.AuthSMTPAddress, StartTLS: cfg.AuthSMTPStartTLS},
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:           health.New(ctx, db.Ping, h),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	stopMaintenance := startDatabaseMaintenance(ctx, db, logger)
	defer stopMaintenance()

	logger.Info("listening", "addr", listener.Addr().String())
	return serveHTTP(ctx, srv, listener, cfg.DrainDelay, cfg.ShutdownTimeout)
}
