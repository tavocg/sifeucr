package handlers

import (
	"net/http"
	"strconv"
	"time"
)

type otpPurpose string

const (
	otpLifetime                      = 10 * time.Minute
	otpPurposeLogin       otpPurpose = "login"
	otpPurposeEmailChange otpPurpose = "email_change"
	otpVerifyLimit                   = 5
	otpVerifyWindow                  = 10 * time.Minute
)

// Budgets are independent of challenge rows: reissuing or consuming an OTP
// must not reset the verification limit. Database failures deny the request.
func (h *Handler) allowOTPRequest(w http.ResponseWriter, r *http.Request, scope, key string, limit int64, window time.Duration) bool {
	retryAfter, err := h.db.Querier().TakeRateLimit(r.Context(), scope, key, time.Now().Unix(), int64(window/time.Second), limit)
	if err != nil {
		h.writeError(
			w,
			r,
			http.StatusInternalServerError,
			err,
			"err.check_otp_limit",
			"failed to check OTP rate limit",
			"scope",
			scope,
		)
		return false
	}
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
		h.writeStatus(
			w,
			r,
			http.StatusTooManyRequests,
			"err.otp_rate_limited",
			"OTP rate limit exceeded",
			"scope",
			scope,
		)
		return false
	}
	return true
}

// Separate budgets prevent both rapid resends and sustained email flooding.
func (h *Handler) allowOTPIssuance(w http.ResponseWriter, r *http.Request, purpose otpPurpose, key string) bool {
	scope := string(purpose)
	return h.allowOTPRequest(w, r, scope+".issue.minute", key, 1, time.Minute) &&
		h.allowOTPRequest(w, r, scope+".issue.window", key, 5, 10*time.Minute)
}

func (h *Handler) allowOTPVerification(w http.ResponseWriter, r *http.Request, purpose otpPurpose, key string) bool {
	return h.allowOTPRequest(w, r, string(purpose)+".verify", key, otpVerifyLimit, otpVerifyWindow)
}

func (h *Handler) signOTP(purpose otpPurpose, identity, code string) string {
	return h.otpSigner.Sign(string(purpose), identity, code)
}
