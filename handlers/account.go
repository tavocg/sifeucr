package handlers

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sifeucr/database"
	"sifeucr/internal/httpx"
	"sifeucr/middleware"
	"sifeucr/routes"
)

type accountResponse struct {
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

func (h *Handler) registerAccountRoutes() {
	authenticated := func(next http.Handler) http.Handler {
		return middleware.AuthenticateBearer(h.logger, h.localize, h.authenticator, next)
	}

	h.AddRoute(routes.Account, authenticated(http.HandlerFunc(h.GetAccount)))
	h.AddRoute(routes.DeleteAccount, authenticated(http.HandlerFunc(h.DeleteAccount)))
	h.AddRoute(routes.Sessions, authenticated(http.HandlerFunc(h.ListSessions)))
	h.AddRoute(routes.RevokeSessions, authenticated(http.HandlerFunc(h.RevokeSessions)))
	h.AddRoute(routes.RevokeSession, authenticated(http.HandlerFunc(h.RevokeSession)))
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
