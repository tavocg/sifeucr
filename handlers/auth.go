package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	auth "github.com/tavocg/go-auth"
	appauth "sifeucr/auth"
	"sifeucr/database"
	"sifeucr/internal/httpx"
	"sifeucr/routes"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) registerAuthRoutes() {
	h.AddRoute(routes.Login, http.HandlerFunc(h.LoginOrCreateAccount))
	h.AddRoute(routes.Refresh, http.HandlerFunc(h.RefreshSession))
}

func (h *Handler) LoginOrCreateAccount(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[loginRequest](h, w, r,
		"err.invalid_login_body",
		"invalid login request body",
	)
	if !ok {
		return
	}

	email := normalizeEmail(req.Email)
	if email == "" {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.invalid_login_email",
			"invalid login email",
		)
		return
	}

	if !h.dev && req.Password == "" {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.missing_password",
			"missing login password",
		)
		return
	}
	if !h.allowLoginAttempt(w, r, email) {
		return
	}
	if !h.dev {
		if err := h.verifyCredentials(r.Context(), h.credentials, email, req.Password); err != nil {
			var smtpError *textproto.Error
			if errors.As(err, &smtpError) && smtpError.Code == 535 {
				h.writeStatus(
					w,
					r,
					http.StatusUnauthorized,
					"err.invalid_credentials",
					"institutional credentials rejected",
				)
			} else {
				h.writeStatus(
					w,
					r,
					http.StatusBadGateway,
					"err.credentials_unavailable",
					"institutional authentication unavailable",
				)
			}
			return
		}
	}
	var subject int64
	if err := h.db.WithTx(r.Context(), func(q database.Querier) error {

		var err error
		subject, err = q.EnsureAccountByEmail(r.Context(), email, time.Now().UTC().Unix())
		if err != nil {
			return err
		}

		if err := q.AssignRoleToAccount(r.Context(), subject, database.RoleDefault); err != nil {
			return err
		}

		return nil
	}); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.finalize_login",
			"failed to finalize credential login",
		)
		return
	}

	accessToken, refreshToken, err := h.authenticator.Issue(r.Context(), &appauth.Claims{
		Sub:       fmt.Sprintf("%d", subject),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.issue_tokens",
			"failed to issue session tokens",
			"sub",
			subject,
		)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newSessionResponse(accessToken, refreshToken))
}

func (h *Handler) RefreshSession(w http.ResponseWriter, r *http.Request) {

	req, ok := decodeBody[refreshRequest](h, w, r,
		"err.invalid_refresh_body",
		"invalid refresh request body",
	)
	if !ok {
		return
	}

	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.missing_refresh",
			"missing refresh token",
		)
		return
	}

	accessToken, refreshToken, err := h.authenticator.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, auth.ErrInvalidToken) || errors.Is(err, auth.ErrExpiredToken) {
			status = http.StatusUnauthorized
		}

		h.writeError(
			w,
			r,
			status,
			err,
			"err.refresh_session",
			"failed to refresh session",
		)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newSessionResponse(accessToken, refreshToken))
}

func newSessionResponse(accessToken *auth.Token, refreshToken *auth.Token) sessionResponse {
	now := time.Now().Unix()
	return sessionResponse{
		AccessToken:           accessToken.Value,
		RefreshToken:          refreshToken.Value,
		ExpiresIn:             expiresIn(accessToken.ExpiresAt, now),
		RefreshTokenExpiresIn: expiresIn(refreshToken.ExpiresAt, now),
	}
}

func expiresIn(expiresAt, now int64) int64 {
	if expiresAt <= now {
		return 0
	}

	return expiresAt - now
}

type sessionResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
}
