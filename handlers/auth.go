package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	auth "github.com/tavocg/go-auth"
	secrets "github.com/tavocg/go-secrets"
	appauth "sifeucr/auth"
	"sifeucr/database"
	"sifeucr/internal/httpx"
	"sifeucr/routes"
)

type loginRequest struct {
	Email string `json:"email"`
}

type verifyRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) registerAuthRoutes() {
	h.AddRoute(routes.Login, http.HandlerFunc(h.LoginOrCreateAccount))
	h.AddRoute(routes.Verify, http.HandlerFunc(h.VerifyAuthenticationCode))
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

	if !h.allowOTPIssuance(w, r, otpPurposeLogin, email) {
		return
	}

	code, err := secrets.RandOTP()
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.generate_login_otp",
			"failed to generate login otp",
		)
		return
	}

	codeHash := h.signOTP(otpPurposeLogin, email, code)
	expiresAt := time.Now().UTC().Add(otpLifetime).Unix()
	if err := h.db.Querier().UpsertAccountLoginRequest(r.Context(), email, codeHash, expiresAt); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.save_login",
			"failed to upsert login request",
		)
		return
	}

	if !h.sendOTP(w, r, email, code, otpPurposeLogin) {
		return
	}

	h.logger.Debug("generated login otp", "email", developmentValue(h.dev, email), "otp", developmentValue(h.dev, code), "expires_at", expiresAt)
	w.WriteHeader(http.StatusNoContent)
}

type sessionResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
}

func (h *Handler) VerifyAuthenticationCode(w http.ResponseWriter, r *http.Request) {

	req, ok := decodeBody[verifyRequest](h, w, r,
		"err.invalid_verify_body",
		"invalid verify request body",
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
			"err.invalid_verify_email",
			"invalid verify email",
		)
		return
	}

	if !h.allowOTPVerification(w, r, otpPurposeLogin, email) {
		return
	}

	if !h.checkOTP(w, r, req.OTP, "err.invalid_login_otp") {
		return
	}

	codeHash := h.signOTP(otpPurposeLogin, email, req.OTP)
	var subject int64
	if err := h.db.WithTx(r.Context(), func(q database.Querier) error {
		if _, err := q.ConsumeAccountLoginRequest(r.Context(), email, codeHash, time.Now().Unix()); err != nil {
			return err
		}

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
		if h.db.IsErrNotFound(err) {
			h.writeStatus(
				w,
				r,
				http.StatusUnauthorized,
				"err.invalid_login_otp",
				"invalid or expired login code",
			)
			return
		}
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.finalize_login",
			"failed to finalize login verification",
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

// developmentValue is only used for bounded, validated email addresses and issued codes.
func developmentValue(dev bool, value string) string {
	if dev {
		return value
	}

	return "******"
}
