// Package health exposes operational probes independently of application routes.
package health

import (
	"context"
	"net/http"
	"time"
)

// New keeps liveness independent of SQL. Cancellation withdraws readiness while
// the server continues accepting requests during endpoint propagation.
func New(ctx context.Context, ping func(context.Context) error, next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		check, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if ctx.Err() != nil || ping(check) != nil || ctx.Err() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", next)
	return mux
}
