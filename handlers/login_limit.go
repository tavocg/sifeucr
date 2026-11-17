package handlers

import (
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) allowLoginAttempt(w http.ResponseWriter, r *http.Request, key string) bool {
	retry, err := h.db.Querier().TakeRateLimit(r.Context(), "login.credentials", key, time.Now().Unix(), 600, 5)
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.login_limit",
			"failed to check login rate limit",
		)
		return false
	}
	if retry > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
		h.writeStatus(
			w,
			r,
			http.StatusTooManyRequests,
			"err.login_rate_limited",
			"login rate limit exceeded",
		)
		return false
	}
	return true
}
