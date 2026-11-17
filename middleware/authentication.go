// Package middleware provides HTTP middleware helpers.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	goauth "github.com/tavocg/go-auth"
	appauth "sifeucr/auth"
	"sifeucr/internal/httpx"
	"sifeucr/internal/logging"
)

type identityContextKey struct{}

// Identity holds verified claims and their validated account subject.
type Identity struct {
	Claims *appauth.Claims
	Sub    int64
}

func AuthenticateBearer(logger logging.Logger, localize LocalizeFunc, authenticator goauth.Authenticator[*appauth.Claims], next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(strings.TrimSpace(r.Header.Get("Authorization")))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			logger.Debug("missing bearer token", logging.RequestArgs(r, http.StatusUnauthorized)...)
			writeJSONError(w, r, localize, http.StatusUnauthorized, "err.missing_bearer")
			return
		}

		claims, err := authenticator.Verify(r.Context(), parts[1])
		if err != nil {
			if errors.Is(err, goauth.ErrInvalidToken) || errors.Is(err, goauth.ErrExpiredToken) {
				logger.Debug("failed to verify bearer token", logging.RequestArgs(r, http.StatusUnauthorized, "error", err)...)
				writeJSONError(w, r, localize, http.StatusUnauthorized, "err.verify_bearer")
				return
			}

			status, key := httpx.ErrorStatus(r.Context(), http.StatusInternalServerError, "err.verify_bearer")
			logger.Error("failed to verify bearer token", logging.RequestArgs(r, status, "error", err)...)
			writeJSONError(w, r, localize, status, key)
			return
		}

		if claims == nil || claims.Subject() == "" {
			logger.Debug("missing authenticated identity", logging.RequestArgs(r, http.StatusUnauthorized)...)
			writeJSONError(w, r, localize, http.StatusUnauthorized, "err.missing_identity")
			return
		}

		sub, err := strconv.ParseInt(claims.Subject(), 10, 64)
		if err != nil || sub <= 0 {
			logger.Debug("invalid authenticated subject", logging.RequestArgs(r, http.StatusUnauthorized, "sub", claims.Subject())...)
			writeJSONError(w, r, localize, http.StatusUnauthorized, "err.invalid_subject")
			return
		}

		identity := Identity{Claims: claims, Sub: sub}
		ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AuthenticatedIdentity(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

type PermissionChecker interface {
	AccountHasPermission(ctx context.Context, sub int64, permissionKey string) (bool, error)
}

func RequirePermission(logger logging.Logger, localize LocalizeFunc, permissions PermissionChecker, permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := AuthenticatedIdentity(r.Context())
		if !ok {
			logger.Debug("missing authenticated identity", logging.RequestArgs(r, http.StatusUnauthorized)...)
			writeJSONError(w, r, localize, http.StatusUnauthorized, "err.missing_identity")
			return
		}

		sub := identity.Sub

		allowed, err := permissions.AccountHasPermission(r.Context(), sub, permission)
		if err != nil {
			status, key := httpx.ErrorStatus(r.Context(), http.StatusInternalServerError, "err.check_role_permission")
			logger.Error("failed to check account permission", logging.RequestArgs(r, status, "sub", sub, "permission", permission, "error", err)...)
			writeJSONError(w, r, localize, status, key)
			return
		}
		if allowed {
			next.ServeHTTP(w, r)
			return
		}

		logger.Debug("permission denied", logging.RequestArgs(r, http.StatusForbidden, "sub", sub, "permission", permission)...)
		writeJSONError(w, r, localize, http.StatusForbidden, "err.permission_denied")
	})
}
