// Package httpx provides the application's JSON request and response conventions.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ErrorStatus selects the deadline error when the request's context has expired.
// Timeouts belonging to a nested operation keep their caller's fallback error.
func ErrorStatus(ctx context.Context, status int, key string) (int, string) {
	if ctx.Err() == context.DeadlineExceeded {
		return http.StatusGatewayTimeout, "err.request_timeout"
	}
	return status, key
}

// DecodeJSON reads one JSON value, rejects unknown fields and limits the body to 1 MiB.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain a single JSON value")
	}

	return nil
}

// WriteJSON sends JSON with no-store so responses are not reused by shared caches.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteJSONError sends the standard localized API error shape.
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
