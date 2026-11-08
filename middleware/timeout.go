package middleware

import (
	"context"
	"net/http"
	"time"
)

// WithTimeout bounds work that honors the request context. It preserves earlier
// deadlines and passes the response writer through without buffering or wrapping.
func WithTimeout(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
