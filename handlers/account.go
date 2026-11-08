package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sifeucr/database"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	secrets "github.com/tavocg/go-secrets"
)

type accountResponse struct {
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

type changeEmailRequest struct {
	NewEmail string `json:"new_email"`
}

type changeEmailConfirmRequest struct {
	OTP string `json:"otp"`
}

func (h *Handler) registerAccountRoutes() {
	authenticated := func(next http.Handler) http.Handler {
		return middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, next)
	}
	canChangeEmail := func(fn http.HandlerFunc) http.Handler {
		return authenticated(middleware.RequirePermission(
			h.logger,
			h.localize,
			h.db.Querier(),
			database.PermissionChangeEmail,
			fn,
		))
	}

	h.AddHandler(http.MethodGet, "/account", authenticated(http.HandlerFunc(h.GetAccount)))
	h.AddHandler(http.MethodDelete, "/account", authenticated(http.HandlerFunc(h.DeleteAccount)))
	h.AddHandler(http.MethodGet, "/account/sessions", authenticated(http.HandlerFunc(h.ListSessions)))
	h.AddHandler(http.MethodDelete, "/account/sessions", authenticated(http.HandlerFunc(h.RevokeSessions)))
	h.AddHandler(http.MethodDelete, "/account/sessions/{id}", authenticated(http.HandlerFunc(h.RevokeSession)))
	h.AddHandler(http.MethodPatch, "/account/email/change", canChangeEmail(h.RequestEmailChange))
	h.AddHandler(http.MethodPatch, "/account/email/change/confirm", canChangeEmail(h.ConfirmEmailChange))
}

func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	sub := identity.Sub

	account, ok := h.loadAccount(w, r, sub)
	if !ok {
		return
	}

	httpx.WriteJSON(w, http.StatusOK, accountResponse{
		Email:     account.Email,
		CreatedAt: account.CreatedAt,
	})
}

func (h *Handler) revokeSessions(w http.ResponseWriter, r *http.Request) (int64, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return 0, false
	}
	sub := identity.Sub

	if err := h.authenticator.RevokeAll(r.Context(), identity.Claims); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.revoke_sessions",
			"failed to revoke account sessions",
			"sub",
			sub,
		)
		return 0, false
	}

	return sub, true
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	sub, ok := h.revokeSessions(w, r)
	if !ok {
		return
	}

	if err := h.db.Querier().DeleteAccount(r.Context(), sub); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.delete_account",
			"failed to delete account",
			"sub",
			sub,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	sub := identity.Sub
	subject := strconv.FormatInt(sub, 10)

	req, ok := decodeBody[changeEmailRequest](h, w, r,
		"err.invalid_email_change_body",
		"invalid account email change request body",
	)
	if !ok {
		return
	}

	newEmail := normalizeEmail(req.NewEmail)
	if newEmail == "" {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.invalid_email_change",
			"invalid account email change target",
			"email",
			req.NewEmail,
			"sub",
			sub,
		)
		return
	}

	account, ok := h.loadAccount(w, r, sub)
	if !ok {
		return
	}

	if account.Email == newEmail {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.email_change_same",
			"account email change target matches current email",
			"sub",
			sub,
			"email",
			newEmail,
		)
		return
	}

	if !h.allowOTPIssuance(w, r, otpPurposeEmailChange, subject) {
		return
	}

	code, err := secrets.RandOTP()
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.generate_email_change_otp",
			"failed to generate account email change otp",
			"sub",
			sub,
		)
		return
	}
	codeHash := h.signOTP(otpPurposeEmailChange, subject, code)
	expiresAt := time.Now().UTC().Add(otpLifetime).Unix()
	if err := h.db.Querier().UpsertAccountEmailChangeRequest(r.Context(), sub, newEmail, codeHash, expiresAt); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.save_email_change",
			"failed to upsert account email change request",
			"sub",
			sub,
			"email",
			newEmail,
		)
		return
	}

	if !h.sendOTP(w, r, newEmail, code, otpPurposeEmailChange) {
		return
	}

	h.logger.Debug("generated account email change otp", "sub", sub, "email", newEmail, "otp", maskedOTP(h.dev, code), "expires_at", expiresAt)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	sub := identity.Sub
	subject := strconv.FormatInt(sub, 10)

	req, ok := decodeBody[changeEmailConfirmRequest](h, w, r,
		"err.invalid_email_confirm_body",
		"invalid account email confirm request body",
	)
	if !ok {
		return
	}

	if !h.allowOTPVerification(w, r, otpPurposeEmailChange, subject) {
		return
	}

	codeHash := h.signOTP(otpPurposeEmailChange, subject, req.OTP)
	if err := h.db.WithTx(r.Context(), func(q database.Querier) error {
		saved, err := q.ConsumeAccountEmailChangeRequest(r.Context(), sub, codeHash, time.Now().Unix())
		if err != nil {
			return err
		}

		return q.UpdateAccountEmail(r.Context(), sub, saved.Email)
	}); err != nil {
		if errors.Is(err, database.ErrEmailUnavailable) {
			h.writeStatus(
				w,
				r,
				http.StatusConflict,
				"err.email_unavailable",
				"account email change target is unavailable",
			)
			return
		}
		if h.db.IsErrNotFound(err) {
			h.writeStatus(
				w,
				r,
				http.StatusUnauthorized,
				"err.invalid_email_change_otp",
				"invalid or expired email change code",
			)
			return
		}
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.confirm_email_change",
			"failed to confirm account email change",
			"sub",
			sub,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Session metadata is descriptive client input. It is never an authentication factor.
type accountSessionResponse struct {
	ID         string `json:"id"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  int64  `json:"created_at"`
	LastUsedAt int64  `json:"last_used_at"`
	ExpiresAt  int64  `json:"expires_at"`
	Current    bool   `json:"current"`
}

type accountSessionsResponse struct {
	Sessions   []accountSessionResponse `json:"sessions"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	beforeTime, beforeID, err := decodeSessionCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.invalid_sessions_cursor",
			"invalid session cursor",
		)
		return
	}
	sessions, err := h.db.Querier().SelectAccountSessions(r.Context(), identity.Sub, time.Now().Unix(), beforeTime, beforeID)
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.select_sessions",
			"failed to select account sessions",
			"sub",
			identity.Sub,
		)
		return
	}
	response := accountSessionsResponse{Sessions: make([]accountSessionResponse, 0, min(len(sessions), 100))}
	if len(sessions) > 100 {
		sessions = sessions[:100]
		last := sessions[len(sessions)-1]
		response.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d.%s", last.CreatedAt, last.SessionID)))
	}
	for _, session := range sessions {
		expiresAt := session.ExpiresAt
		if expiresAt != 0 {
			expiresAt = max(expiresAt, session.AccessExpiresAt)
		}
		response.Sessions = append(response.Sessions, accountSessionResponse{
			ID:         session.SessionID,
			UserAgent:  session.UserAgent,
			CreatedAt:  session.CreatedAt,
			LastUsedAt: session.LastUsedAt,
			ExpiresAt:  expiresAt,
			Current:    session.SessionID == identity.Claims.SessionID,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) RevokeSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.revokeSessions(w, r); !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if !validSessionID(id) {
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			"err.invalid_session_id",
			"invalid session identifier",
		)
		return
	}
	// Match issuance/refresh/revocation's lock order, so rotating a refresh token
	// cannot replace the row while this operation is deleting that session.
	if err := h.db.WithTx(r.Context(), func(q database.Querier) error {
		if _, err := q.LockAccountAuthID(r.Context(), identity.Sub); err != nil {
			if h.db.IsErrNotFound(err) {
				return nil
			}
			return err
		}
		return q.DeleteAccountSession(r.Context(), identity.Sub, id)
	}); err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.revoke_session",
			"failed to revoke account session",
			"sub",
			identity.Sub,
		)
		return
	}
	h.logger.Debug("revoked account session", "sub", identity.Sub, "session_id", id)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func validSessionID(id string) bool {
	return len(id) == 64 && strings.Trim(id, "0123456789abcdef") == ""
}

func decodeSessionCursor(cursor string) (int64, string, error) {
	if cursor == "" {
		return math.MaxInt64, "", nil
	}
	if len(cursor) <= 128 {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			stamp, id, _ := strings.Cut(string(decoded), ".")
			createdAt, err := strconv.ParseInt(stamp, 10, 64)
			if err == nil && createdAt >= 0 && validSessionID(id) {
				return createdAt, id, nil
			}
		}
	}
	return 0, "", fmt.Errorf("invalid session cursor")
}
