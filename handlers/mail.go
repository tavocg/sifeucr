package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/tavocg/go-email/mailer"
)

// Persist the challenge before sending: a delivered code must already be usable.
// Failed or ambiguous delivery does not delete it or reset the request budget.
func (h *Handler) sendOTP(w http.ResponseWriter, r *http.Request, to, code string, purpose otpPurpose) bool {
	data := struct {
		Code    string
		Minutes int
	}{code, int(otpLifetime / time.Minute)}
	message := mailer.NewPlainMessage(
		h.mailFrom,
		[]string{to},
		h.localize(r, "mail."+string(purpose)+".subject"),
		h.localize(r, "mail."+string(purpose)+".body", data),
	)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.mailer.Send(ctx, message); err != nil {
		h.writeError(
			w,
			r,
			http.StatusBadGateway,
			err,
			"err.send_otp",
			"failed to send verification email",
			"purpose",
			purpose,
		)
		return false
	}
	return true
}
