package handlers

import (
	"net/http"

	email "github.com/tavocg/go-email"
	"sifeucr/database"
	"sifeucr/internal/httpx"
	"sifeucr/internal/logging"
	"sifeucr/middleware"
)

func (h *Handler) writeStatus(w http.ResponseWriter, r *http.Request, status int, key string, msg string, args ...any) {
	status, key = httpx.ErrorStatus(r.Context(), status, key)
	logArgs := logging.RequestArgs(r, status, args...)

	if status >= http.StatusInternalServerError {
		h.logger.Error(msg, logArgs...)
	} else if status >= http.StatusBadRequest {
		h.logger.Debug(msg, logArgs...)
	}

	httpx.WriteJSONError(w, status, h.localize(r, key))
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, err error, key string, msg string, args ...any) {
	if err != nil {
		args = append([]any{"error", err}, args...)
	}
	h.writeStatus(w, r, status, key, msg, args...)
}

func (h *Handler) localize(r *http.Request, key string, args ...any) string {
	if h.localizer == nil {
		return key
	}

	L := h.localizer.LocalizerFunc(h.localizer.PickLanguageFromRequest(r))
	return L(key, args...)
}

// language returns the negotiated request language. It falls back to English
// when no localizer is configured or no offered language is acceptable, which
// matches the translation lookup order.
func (h *Handler) language(r *http.Request) string {
	const fallback = "en"
	if h.localizer == nil {
		return fallback
	}

	if language := h.localizer.PickLanguageFromRequest(r); language != "" {
		return language
	}
	return fallback
}

// normalizeEmail returns normalized email address if it meets strict rules.
// Returns an empty string for invalid input.
func normalizeEmail(original string) string {
	valid, err := email.StrictParser(original)
	if err != nil {
		return ""
	}

	valid.Normalize()
	return valid.Address()
}

func (h *Handler) requireIdentity(w http.ResponseWriter, r *http.Request) (middleware.Identity, bool) {
	identity, ok := middleware.AuthenticatedIdentity(r.Context())
	if !ok {
		h.writeStatus(
			w,
			r,
			http.StatusInternalServerError,
			"err.missing_account_subject",
			"missing authenticated account subject",
		)
	}
	return identity, ok
}

func decodeBody[T any](h *Handler, w http.ResponseWriter, r *http.Request, key, message string) (T, bool) {
	var body T
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		// Decoder errors can contain arbitrary field names supplied by the client.
		h.writeStatus(
			w,
			r,
			http.StatusBadRequest,
			key,
			message,
		)
		return body, false
	}
	return body, true
}

func (h *Handler) loadAccount(w http.ResponseWriter, r *http.Request, sub int64) (*database.Account, bool) {
	account, err := h.db.Querier().SelectAccountBySub(r.Context(), sub)
	if err != nil {
		status, key, message := http.StatusInternalServerError, "err.select_account", "failed to select account"
		if h.db.IsErrNotFound(err) {
			status, key, message = http.StatusNotFound, "err.account_not_found", "account not found"
		}
		h.writeError(
			w,
			r,
			status,
			err,
			key,
			message,
			"sub",
			sub,
		)
		return nil, false
	}
	return account, true
}
